import argparse
import os
import joblib
import numpy as np
import pandas as pd
from sklearn.model_selection import train_test_split
from sklearn.metrics import roc_auc_score
from sklearn.linear_model import LogisticRegression
from lightgbm import LGBMClassifier
import json


def make_features(df: pd.DataFrame) -> pd.DataFrame:
    df = df.copy()
    df["ret"] = np.log(df["close"]).diff().fillna(0)
    df["ret_1"] = df["ret"].shift(1).fillna(0)
    df["ret_5"] = df["ret"].rolling(5).sum().fillna(0)
    df["sigma_60"] = df["ret"].rolling(60).std().fillna(0)
    return df


def triple_barrier_labels(close: pd.Series, horizon: int, up: float, down: float) -> pd.Series:
    n = len(close)
    labels = np.zeros(n)
    for i in range(n):
        end = min(n - 1, i + horizon)
        path = close.iloc[i:end+1].values
        base = path[0]
        rel = path / base - 1.0
        if rel.max() >= up:
            labels[i] = 1
        elif rel.min() <= -down:
            labels[i] = 0
        else:
            labels[i] = 1 if rel[-1] > 0 else 0
    return pd.Series(labels, index=close.index)


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--data", required=True)
    p.add_argument("--outdir", default="./python/models/edge_score/v1")
    p.add_argument("--horizon", type=int, default=15)
    p.add_argument("--up", type=float, default=0.002)
    p.add_argument("--down", type=float, default=0.002)
    args = p.parse_args()

    os.makedirs(args.outdir, exist_ok=True)
    df = pd.read_parquet(args.data)
    df = make_features(df)
    df["label"] = triple_barrier_labels(df["close"], horizon=args.horizon, up=args.up, down=args.down)
    df = df.dropna()

    feats = ["ret_1", "ret_5", "sigma_60"]
    X = df[feats].values
    y = df["label"].values

    X_train, X_val, y_train, y_val = train_test_split(X, y, test_size=0.2, shuffle=False)
    model = LGBMClassifier(n_estimators=400, max_depth=-1, learning_rate=0.03)
    model.fit(X_train, y_train)
    proba_val = model.predict_proba(X_val)[:, 1]
    auc = roc_auc_score(y_val, proba_val)

    # Platt scaling (calibration)
    calibrator = LogisticRegression(max_iter=1000)
    calibrator.fit(proba_val.reshape(-1, 1), y_val)

    joblib.dump(model, os.path.join(args.outdir, "model.joblib"))
    joblib.dump(calibrator, os.path.join(args.outdir, "calib.joblib"))
    with open(os.path.join(args.outdir, "feats.json"), "w") as f:
        json.dump({"features": feats}, f)
    with open(os.path.join(args.outdir, "metrics.txt"), "w") as f:
        f.write(f"roc_auc_raw: {auc:.4f}\n")


if __name__ == "__main__":
    main()


