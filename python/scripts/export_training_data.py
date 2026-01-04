import argparse
import os
import pandas as pd
from clickhouse_driver import Client


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--host", default="localhost")
    p.add_argument("--port", type=int, default=9000)
    p.add_argument("--database", default="finpull")
    p.add_argument("--user", default="default")
    p.add_argument("--password", default="")
    p.add_argument("--symbol", required=True)
    p.add_argument("--from", dest="from_ts", required=True)
    p.add_argument("--to", dest="to_ts", required=True)
    p.add_argument("--out", default="./data/training/candles.parquet")
    args = p.parse_args()

    client = Client(host=args.host, port=args.port, database=args.database, user=args.user, password=args.password)
    q = (
        "SELECT bucket, symbol, open, high, low, close, vol FROM finpull.rt_candles_1m "
        "WHERE symbol=%(symbol)s AND bucket >= %(from)s AND bucket <= %(to)s ORDER BY bucket ASC"
    )
    rows = client.execute(q, {"symbol": args.symbol, "from": args.from_ts, "to": args.to_ts})
    df = pd.DataFrame(rows, columns=["bucket","symbol","open","high","low","close","vol"])
    os.makedirs(os.path.dirname(args.out), exist_ok=True)
    df.to_parquet(args.out, index=False)


if __name__ == "__main__":
    main()


