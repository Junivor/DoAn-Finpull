from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import List, Optional
import numpy as np
from sklearn.mixture import GaussianMixture
import os
import joblib
import json

app = FastAPI(title="Regime Service", version="1.0.0")

MODEL_PATH = os.getenv("REGIME_MODEL_PATH", "./python/models/regime/v1/model.joblib")
METADATA_PATH = os.getenv("REGIME_METADATA_PATH", "./python/models/regime/v1/metadata.json")

model: Optional[GaussianMixture] = None
n_states: int = 3
state_names: List[str] = ["quiet", "normal", "volatile"]


def _compute_realized_vol(returns: List[float], window: int = 60) -> float:
    """Compute realized volatility from returns."""
    if len(returns) < 2:
        return 0.0
    if len(returns) < window:
        window = len(returns)
    recent = returns[-window:]
    return float(np.std(recent) * np.sqrt(252.0 * window))  # Annualized


def _lazy_load():
    """Lazy load HMM model if available."""
    global model, n_states, state_names
    if model is None and os.path.exists(MODEL_PATH):
        try:
            model = joblib.load(MODEL_PATH)
            if hasattr(model, "n_components"):
                n_states = model.n_components
        except Exception:
            pass
    
    if os.path.exists(METADATA_PATH):
        try:
            with open(METADATA_PATH, "r") as f:
                meta = json.load(f)
                if "n_states" in meta:
                    n_states = meta["n_states"]
                if "state_names" in meta:
                    state_names = meta["state_names"]
        except Exception:
            pass


def _detect_regime_bocpd(returns: List[float]) -> tuple[str, List[float], float]:
    """
    Simplified Bayesian Online Change Point Detection (BOCPD) inspired approach.
    Uses realized volatility clustering to detect regime changes.
    """
    if len(returns) < 10:
        return "quiet", [1.0, 0.0, 0.0], 0.33
    
    # Compute rolling realized volatility
    window = min(60, len(returns))
    recent_vol = _compute_realized_vol(returns, window)
    
    # Threshold-based regime detection (can be replaced with proper BOCPD)
    if recent_vol < 0.15:
        state_idx = 0  # quiet
        probs = [0.7, 0.2, 0.1]
    elif recent_vol < 0.35:
        state_idx = 1  # normal
        probs = [0.2, 0.7, 0.1]
    else:
        state_idx = 2  # volatile
        probs = [0.1, 0.2, 0.7]
    
    state = state_names[state_idx] if state_idx < len(state_names) else "normal"
    confidence = max(probs)
    return state, probs, confidence


def _detect_regime_hmm(returns: List[float]) -> tuple[str, List[float], float]:
    """Detect regime using trained HMM model."""
    global model, n_states, state_names
    
    if model is None or len(returns) < n_states:
        # Fallback to BOCPD
        return _detect_regime_bocpd(returns)
    
    # Compute features: returns and realized volatility
    features = []
    for i in range(len(returns)):
        window = min(20, i + 1)
        vol = _compute_realized_vol(returns[:i+1], window) if i > 0 else 0.0
        features.append([returns[i], vol])
    
    X = np.array(features)
    
    try:
        # Predict most likely state sequence
        states = model.predict(X)
        # Predict probabilities for last state
        probs = model.predict_proba(X[-1:])[0].tolist()
        
        # Most likely state
        state_idx = int(np.argmax(probs))
        state = state_names[state_idx] if state_idx < len(state_names) else "normal"
        confidence = float(probs[state_idx])
        
        # Ensure probs match n_states
        while len(probs) < n_states:
            probs.append(0.0)
        probs = probs[:n_states]
        
        return state, probs, confidence
    except Exception as e:
        # Fallback on error
        return _detect_regime_bocpd(returns)


class RegimeRequest(BaseModel):
    symbol: str
    returns: List[float]


class RegimeResponse(BaseModel):
    state: str
    prob: List[float]
    confidence: float


@app.get("/health")
def health():
    return {"status": "ok", "model_loaded": model is not None}


@app.post("/regime/detect", response_model=RegimeResponse)
def detect(req: RegimeRequest):
    """Detect market regime from returns using HMM (if available) or BOCPD fallback."""
    _lazy_load()
    
    if not req.returns:
        raise HTTPException(status_code=400, detail="returns list cannot be empty")
    
    if len(req.returns) < 2:
        return RegimeResponse(
            state="quiet",
            prob=[1.0, 0.0, 0.0] if n_states >= 3 else [1.0],
            confidence=1.0
        )
    
    # Use HMM if model loaded, otherwise BOCPD
    if model is not None:
        state, probs, confidence = _detect_regime_hmm(req.returns)
    else:
        state, probs, confidence = _detect_regime_bocpd(req.returns)
    
    # Normalize probs to match n_states
    while len(probs) < n_states:
        probs.append(0.0)
    probs = probs[:n_states]
    total = sum(probs)
    if total > 0:
        probs = [p / total for p in probs]
    
    return RegimeResponse(state=state, prob=probs, confidence=confidence)
