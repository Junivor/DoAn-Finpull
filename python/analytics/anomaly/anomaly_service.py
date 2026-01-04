from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import List, Optional
import numpy as np
from statsmodels.tsa.seasonal import STL
from scipy import stats

app = FastAPI(title="Anomaly Service", version="1.0.0")

# Generalized ESD test parameters
ESD_ALPHA = 0.05  # Significance level
ESD_MAX_OUTLIERS = 10  # Maximum outliers to detect


def _stl_decompose(data: np.ndarray, period: Optional[int] = None) -> np.ndarray:
    """STL decomposition to extract trend and residuals."""
    if len(data) < 2 * (period or 10):
        # Too short for STL, use simple detrending
        trend = np.mean(data)
        return data - trend
    
    try:
        if period is None:
            # Auto-detect period (try daily if hourly, etc.)
            period = min(20, max(5, len(data) // 10))
        
        stl = STL(data, period=period, robust=True)
        result = stl.fit()
        # Return residuals (trend + seasonal removed)
        return result.resid
    except Exception:
        # Fallback: simple detrending
        trend = np.mean(data)
        return data - trend


def _generalized_esd(data: np.ndarray, alpha: float = ESD_ALPHA, max_outliers: int = ESD_MAX_OUTLIERS) -> List[int]:
    """
    Generalized Extreme Studentized Deviate (ESD) test for outliers.
    Returns indices of detected anomalies.
    """
    if len(data) < 3:
        return []
    
    data_copy = data.copy()
    n = len(data_copy)
    outliers: List[int] = []
    
    # Normalize data
    mean = np.mean(data_copy)
    std = np.std(data_copy)
    if std == 0:
        return []
    
    normalized = (data_copy - mean) / std
    
    for i in range(max_outliers):
        if len(normalized) < 1:
            break
        
        # Find maximum absolute deviation
        abs_deviations = np.abs(normalized)
        max_idx = int(np.argmax(abs_deviations))
        max_val = abs_deviations[max_idx]
        
        # Critical value for ESD test
        # t-statistic for (n - i - 2) degrees of freedom
        df = n - i - 2
        if df < 1:
            break
        
        # Approximate critical value using t-distribution
        t_critical = stats.t.ppf(1 - alpha / (2 * (n - i)), df)
        lambda_i = ((n - i - 1) * t_critical) / np.sqrt((df) * (1 + t_critical ** 2))
        
        if max_val > lambda_i:
            # Found outlier
            original_idx = max_idx
            outliers.append(original_idx)
            
            # Remove from consideration
            normalized = np.delete(normalized, max_idx)
        else:
            break
    
    return outliers


def _robust_zscore(data: np.ndarray, threshold: float = 3.0) -> List[int]:
    """
    Robust z-score using median and MAD (Median Absolute Deviation).
    More robust to outliers than standard z-score.
    """
    if len(data) < 3:
        return []
    
    median = np.median(data)
    mad = np.median(np.abs(data - median))
    
    if mad == 0:
        return []
    
    # Modified z-score
    modified_z_scores = 0.6745 * (data - median) / mad
    anomalies = np.where(np.abs(modified_z_scores) > threshold)[0].tolist()
    
    return anomalies


def _detect_shocks(returns: List[float], vols: List[float]) -> List[dict]:
    """
    Detect market shocks using STL decomposition + Generalized ESD / robust z-score.
    """
    if len(returns) < 10 or len(vols) < 10:
        return []
    
    returns_arr = np.array(returns)
    vols_arr = np.array(vols)
    
    # Combine returns and volatility into a shock signal
    # Normalize both to similar scales
    if np.std(returns_arr) > 0:
        returns_norm = (returns_arr - np.mean(returns_arr)) / np.std(returns_arr)
    else:
        returns_norm = returns_arr
    
    if np.std(vols_arr) > 0:
        vols_norm = (vols_arr - np.mean(vols_arr)) / np.std(vols_arr)
    else:
        vols_norm = vols_arr
    
    # Combine: shock = abs(return) * volatility
    shock_signal = np.abs(returns_norm) * (1.0 + vols_norm)
    
    # Apply STL to extract anomalies from trend/seasonality
    try:
        residuals = _stl_decompose(shock_signal, period=None)
    except Exception:
        residuals = shock_signal
    
    # Detect anomalies using Generalized ESD
    esd_outliers = _generalized_esd(residuals, alpha=ESD_ALPHA, max_outliers=ESD_MAX_OUTLIERS)
    
    # Also use robust z-score as secondary check
    zscore_outliers = _robust_zscore(residuals, threshold=3.5)
    
    # Combine both methods (union)
    all_outliers = list(set(esd_outliers + zscore_outliers))
    
    # Classify anomalies
    anomalies = []
    for idx in all_outliers:
        if idx >= len(returns):
            continue
        
        ret_val = returns[idx]
        vol_val = vols[idx] if idx < len(vols) else 0.0
        
        # Classify type
        if abs(ret_val) > 2.0 * np.std(returns_arr):
            anom_type = "price_shock"
            severity = min(1.0, abs(ret_val) / (3.0 * np.std(returns_arr)))
        elif vol_val > 2.0 * np.std(vols_arr):
            anom_type = "volatility_spike"
            severity = min(1.0, vol_val / (3.0 * np.std(vols_arr)))
        else:
            anom_type = "anomaly"
            severity = 0.5
        
        anomalies.append({
            "ts_index": int(idx),
            "type": anom_type,
            "severity": float(severity)
        })
    
    # Sort by index
    anomalies.sort(key=lambda x: x["ts_index"])
    
    return anomalies


class AnomalyRequest(BaseModel):
    symbol: str
    returns: List[float]
    vols: List[float]


class Anomaly(BaseModel):
    ts_index: int
    type: str
    severity: float


class AnomalyResponse(BaseModel):
    anomalies: List[Anomaly]


@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/anomaly/detect", response_model=AnomalyResponse)
def detect(req: AnomalyRequest):
    """Detect market anomalies using STL + Generalized ESD / robust z-score."""
    if not req.returns:
        raise HTTPException(status_code=400, detail="returns list cannot be empty")
    
    if not req.vols:
        raise HTTPException(status_code=400, detail="vols list cannot be empty")
    
    if len(req.returns) != len(req.vols):
        raise HTTPException(
            status_code=400,
            detail=f"returns and vols must have same length (got {len(req.returns)} vs {len(req.vols)})"
        )
    
    if len(req.returns) < 10:
        return AnomalyResponse(anomalies=[])
    
    anomalies_dict = _detect_shocks(req.returns, req.vols)
    anomalies = [Anomaly(**a) for a in anomalies_dict]
    
    return AnomalyResponse(anomalies=anomalies)
