from fastapi import FastAPI
from pydantic import BaseModel
from typing import Dict, List
import os
import json
import joblib
import numpy as np

app = FastAPI(title="Edge Score Service", version="0.1.0")

MODEL_PATH = os.getenv("EDGE_MODEL_PATH", "./python/models/edge_score/v1/model.joblib")
CALIB_PATH = os.getenv("EDGE_CALIB_PATH", "./python/models/edge_score/v1/calib.joblib")
FEATS_PATH = os.getenv("EDGE_FEATS_PATH", "./python/models/edge_score/v1/feats.json")

model = None
calibrator = None
feat_order: List[str] = ["ret_1", "ret_5", "sigma_60"]


def _lazy_load():
    global model, calibrator, feat_order
    if model is None and os.path.exists(MODEL_PATH):
        model = joblib.load(MODEL_PATH)
    if calibrator is None and os.path.exists(CALIB_PATH):
        calibrator = joblib.load(CALIB_PATH)
    if os.path.exists(FEATS_PATH):
        try:
            with open(FEATS_PATH, "r") as f:
                meta = json.load(f)
                if "features" in meta:
                    feat_order = meta["features"]
        except Exception:
            pass


class EdgeRequest(BaseModel):
    symbol: str
    features: Dict[str, float]
    horizon: str  # "15m"


class EdgeResponse(BaseModel):
    proba_up: float
    regime: str
    sigma: float
    confidence: float


@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/edge/predict", response_model=EdgeResponse)
def predict(req: EdgeRequest):
    _lazy_load()
    # Build feature vector according to training order
    x = np.array([[req.features.get(k, 0.0) for k in feat_order]], dtype=float)
    if model is None:
        base_proba = 0.5
    else:
        base_proba = float(model.predict_proba(x)[:, 1][0])
    if calibrator is not None:
        proba = float(calibrator.predict_proba(np.array(base_proba).reshape(-1, 1))[:, 1][0])
    else:
        proba = base_proba

    sigma = float(req.features.get("sigma_60", 0.0))
    regime = req.features.get("regime", "unknown") if isinstance(req.features.get("regime", None), str) else "unknown"
    confidence = abs(proba - 0.5) * 2.0
    return EdgeResponse(proba_up=proba, regime=regime, sigma=sigma, confidence=confidence)


