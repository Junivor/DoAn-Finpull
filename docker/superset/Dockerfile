FROM apache/superset:3.0.0

USER root

# Install ClickHouse driver for Superset
RUN pip install --no-cache-dir \
    clickhouse-sqlalchemy==0.2.4 \
    clickhouse-connect==0.7.6 \
    psycopg2-binary==2.9.9

# Copy startup script and setup scripts
COPY start.sh /usr/local/bin/start-superset.sh
COPY setup_clickhouse.py /app/setup_clickhouse.py
RUN chmod +x /usr/local/bin/start-superset.sh && \
    chown superset:superset /usr/local/bin/start-superset.sh /app/setup_clickhouse.py

USER superset

CMD ["/usr/local/bin/start-superset.sh"]
