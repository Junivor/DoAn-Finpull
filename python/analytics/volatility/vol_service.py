from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import Dict, Optional
import numpy as np
from arch import arch_model
import os
import joblib
import json

app = FastAPI(title="Volatility Service", version="1.0.0")

MODEL_PATH = os.getenv("VOL_MODEL_PATH", "./python/models/volatility/v1/model.joblib")
METADATA_PATH = os.getenv("VOL_METADATA_PATH", "./python/models/volatility/v1/metadata.json")

garch_model: Optional[any] = None
lgbm_model: Optional[any] = None
use_lgbm: bool = False


def _parse_horizon(horizon: str) -> int:
    """Parse horizon string to number of steps ahead."""
    horizon_lower = horizon.lower().strip()
    if horizon_lower.endswith("m"):
        minutes = int(horizon_lower[:-1])
        # Assuming 1-minute bars, return number of steps
        return minutes
    elif horizon_lower.endswith("h"):
        hours = int(horizon_lower[:-1])
        return hours * 60
    elif horizon_lower.endswith("d"):
        days = int(horizon_lower[:-1])
        return days * 24 * 60
    return 5  # default 5 minutes


def _garch_forecast(nowcast_sigma: float, horizon_steps: int) -> tuple[float, float]:
    """
    GARCH(1,1) volatility forecast.
    Simplified: uses nowcast as recent volatility and projects forward.
    For full GARCH, would fit model on historical returns.
    """
    if nowcast_sigma <= 0:
        nowcast_sigma = 0.02  # Default 2% annualized
    
    # Simplified GARCH forecast: mean-reverting to long-term volatility
    long_term_vol = 0.20  # 20% annualized (adjust based on asset)
    alpha = 0.05  # Mean reversion speed
    persistence = 0.95
    
    # Forecast: mean-reverting process
    forecast = long_term_vol + (nowcast_sigma - long_term_vol) * (persistence ** horizon_steps)
    
    # Ensure reasonable bounds
    forecast = max(0.01, min(1.0, forecast))
    nowcast_sigma = max(0.01, min(1.0, nowcast_sigma))
    
    return float(nowcast_sigma), float(forecast)


def _lgbm_forecast(features: Dict[str, float], horizon: str) -> tuple[float, float]:
    """LightGBM-based volatility forecast."""
    global lgbm_model, use_lgbm
    
    if lgbm_model is None:
        return _garch_forecast(features.get("nowcast_sigma", 0.02), _parse_horizon(horizon))
    
    try:
        # Build feature vector (must match training order)
        feat_order = [
            "nowcast_sigma", "ret_1", "ret_5", "ret_20",
            "volume_ma", "high_low_range"
        ]
        
        x = np.array([[features.get(k, 0.0) for k in feat_order]], dtype=float)
        
        if hasattr(lgbm_model, "predict"):
            forecast = float(lgbm_model.predict(x)[0])
        else:
            return _garch_forecast(features.get("nowcast_sigma", 0.02), _parse_horizon(horizon))
        
        nowcast = float(features.get("nowcast_sigma", forecast * 0.9))
        
        # Ensure reasonable bounds
        forecast = max(0.01, min(1.0, forecast))
        nowcast = max(0.01, min(1.0, nowcast))
        
        return nowcast, forecast
    except Exception:
        # Fallback to GARCH
        return _garch_forecast(features.get("nowcast_sigma", 0.02), _parse_horizon(horizon))


def _lazy_load():
    """Lazy load models if available."""
    global garch_model, lgbm_model, use_lgbm
    
    if lgbm_model is None and os.path.exists(MODEL_PATH):
        try:
            loaded = joblib.load(MODEL_PATH)
            # Could be dict with 'garch' and 'lgbm' keys, or single model
            if isinstance(loaded, dict):
                lgbm_model = loaded.get("lgbm")
                garch_model = loaded.get("garch")
            else:
                # Assume LightGBM if single model
                lgbm_model = loaded
                use_lgbm = True
        except Exception:
            pass
    
    if os.path.exists(METADATA_PATH):
        try:
            with open(METADATA_PATH, "r") as f:
                meta = json.load(f)
                use_lgbm = meta.get("use_lgbm", False)
        except Exception:
            pass


class VolRequest(BaseModel):
    symbol: str
    features: Dict[str, float]
    horizon: str  # "5m" or "30m"


class VolResponse(BaseModel):
    forecast: float
    nowcast: float
    model: str


@app.get("/health")
def health():
    return {
        "status": "ok",
        "garch_available": True,
        "lgbm_loaded": lgbm_model is not None
    }


@app.post("/vol/forecast", response_model=VolResponse)
def forecast(req: VolRequest):
    """Forecast volatility using GARCH(1,1) + optional LightGBM enhancement."""
    _lazy_load()
    
    if not req.features:
        raise HTTPException(status_code=400, detail="features dictionary cannot be empty")
    
    nowcast_sigma = req.features.get("nowcast_sigma", 0.02)
    horizon_steps = _parse_horizon(req.horizon)
    
    # Try LightGBM if available, otherwise GARCH
    if lgbm_model is not None and use_lgbm:
        nowcast, forecast = _lgbm_forecast(req.features, req.horizon)
        model_name = "GARCH+LightGBM"
    else:
        nowcast, forecast = _garch_forecast(nowcast_sigma, horizon_steps)
        model_name = "GARCH(1,1)"
    
    return VolResponse(forecast=forecast, nowcast=nowcast, model=model_name)
