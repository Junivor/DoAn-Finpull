# FinPull - Real-time Market Data Collector
[Xem hướng dẫn cài đặt và sử dụng dành cho tiếng Việt](HuongDanCaiDatVaSuDung.md) 

Clean Architecture + Dependency Injection (Wire) implementation for collecting real-time market data from Finnhub and streaming to Kafka/ClickHouse.

## System Architect

![img.png](img/img.png)

## Architecture

```
pkg/                          # Shared infrastructure layer
├── server/app.go              → Application lifecycle (Start/Stop/Health/Ready)
├── clickhouse/client.go       → Connection pool manager + schema initialization
├── kafka/client.go            → Writer manager + batch/compression config
├── config/config.go           → YAML config loader + validation
└── metrics/prometheus.go      → Prometheus metrics recorder

internal/
├── domain/
│   ├── models/trade.go        → Business entities
│   └── repository/            → Port interfaces (abstractions)
│       └── interfaces.go      → MarketStream, Publisher, Storage, Metrics
├── repository/                → Adapters (interface implementations)
│   ├── clickhouse/storage.go  → ClickHouse storage adapter
│   └── kafka/publisher.go     → Kafka publisher adapter
├── service/
│   └── finnhub/client.go      → Finnhub WebSocket client (third-party)
├── usecase/                   → Business logic
│   ├── trade_processor.go     → Process & route trades to backend
│   └── trade_collector.go     → Collect trades from market stream
└── di/                        → Dependency injection
    ├── providers.go           → Provider functions for Wire
    └── wire.go                → Wire injector definition

cmd/app/main.go                → Entry point (3-step: load config → wire DI → run)
```

## Key Principles

### 1. **Clean Architecture**
- **Domain layer**: Pure business logic, no external dependencies
- **Use case layer**: Application-specific business rules
- **Interface adapters**: Convert data between domain and external systems
- **Infrastructure**: Frameworks, tools, drivers (ClickHouse, Kafka, HTTP)

### 2. **Dependency Injection with Wire**
- All dependencies injected via Wire
- No global state or singletons
- Easy to test and mock

### 3. **Separation of Concerns**
- **pkg/**: Infrastructure setup (connections, configs) - REUSABLE
- **internal/**: Application logic (domain, usecases, adapters) - BUSINESS SPECIFIC
- **cmd/**: Entry points - MINIMAL (3 steps)

### 4. **Connection Management**
- Connections created **ONCE** in `pkg/`
- Repositories receive connections, never create them
- Cleanup handled centrally

## Quick Start

### Build - IMPORTANT STEP NEED TO DO BEFORE RUNNING

````bash
# build for linux OS
make build-linux
# or for windows OS
make build-window
````


### Configuration

`config/dev.yaml`:
```yaml
environment: development
backend:
  type: kafka  # or clickhouse
finnhub:
  api_key: ${FINNHUB_API_KEY}
  symbols:
    - BINANCE:BTCUSDT
kafka:
  brokers:
    - kafka-headless:9092
  topic: finpull
```

Environment variables override YAML:
```bash
export FINNHUB_API_KEY=your_key
export BACKEND=clickhouse
./bin/finpull
```

## Development

### Install Tools

```bash
make install-tools
```

Installs:
- Wire (dependency injection code generator)
- golangci-lint (linter)

### Generate Wire Code

```bash
make wire
```

Generates `internal/di/wire_gen.go`.

### Lint

```bash
make lint
```

Enforces:
- Cyclomatic complexity < 15
- Cognitive complexity < 15
- Function length < 100 lines
- Error handling
- Code style

### Test

```bash
make test
```

## Run With Docker

### Start Infrastructure

```bash
make docker-up
```

Starts:
- ClickHouse (`:8123`, `:9000`) UI: `http://localhost:9000`
- Kafka (`:9092`)
- Zookeeper
- Postgres (`:5432`)
- Redis (`:6379`)
- Superset (`:8088`)
- Prometheus (`:9090`)
- Grafana (`:3000`) UI: `http://localhost:3000`

### Stop Infrastructure

```bash
make docker-down
```

## Additional Setup
### Setup Superset connection to ClickHouse
1. Open Superset at `http://localhost:8088`
![img_1.png](img/img_1.png)
2. Login with default credentials (`admin`/`admin`)
![img_2.png](img/img_2.png)
3. Go to `Settings` -> `Database connection`
![img_3.png](img/img_3.png)
4. Go to `Supporting databases`
![img_4.png](img/img_4.png)
5. Choose ``ClickHouse Connect (Superset)``
![img_5.png](img/img_5.png)
6. Choose `Connect this database with a SQLAlchemy URI string instead`
![img_6.png](img/img_6.png)
7. Use SQLAlchemy URI: `clickhouse+native://default@clickhouse:9000/finpull` -> `Test Connection` -> `Success` -> `Connect`
![img_6.1.png](img/img_6.1.png)
![img_6.2.png](img/img_6.2.png)

### Import boilerplate dataset
1. Choose `Dataset` on the Navigation panel
![img_7.png](img/img_7.png)
2. Click the `import icon` on the top right next to the `+ Dataset` button
![img_8.png](img/img_8.png)
3. At the `Import dataset` modal, click the `SELECT FILE`
![img_9.png](img/img_9.png)
4. Choose the boilerplate dataset file that included when you cloned the repo: `Finpull/docker/superset/import/dataset_export_20260101T144629` -> `Select`
![img_10.png](img/img_10.png)
5. After the file is selected, click the `IMPORT` button at the bottom right of the modal
![img_11.png](img/img_11.png)
6. You should see the `finpull_trades` dataset in the dataset list now `(Note that it may take a while to import depending on your machine performance, if you don't see it try refreshing the page or try adding it manually)`
![img_12.png](img/img_12.png)

### Import boilerplate
1. Choose `Charts` on the Navigation panel
![img_13.png](img/img_13.png)
2. Click the `import icon` on the top right next to the `+ Chart` button
![img_14.png](img/img_14.png)
3. At the `Import chart` modal, click the `SELECT FILE`
![img_15.png](img/img_15.png)
4. Choose the boilerplate chart file that included when you cloned the repo: `Finpull/docker/superset/import/chart_export_20260101T144638` -> `Select`
![img_16.png](img/img_16.png)
5. After the file is selected, click the `IMPORT` button at the bottom right of the modal
![img_17.png](img/img_17.png)
6. You should see multiple charts in the chart list now `(Note that it may take a while to import depending on your machine performance, if you don't see them try refreshing the page or try adding them manually)`
![img_18.png](img/img_18.png)

### Import boilerplate dashboards dashboard_export_20260101T144643
1. Choose `Dashboards` on the Navigation panel
![img_19.png](img/img_19.png)
2. Click the `import icon` on the top right next to the `+ Dashboard` button
![img_20.png](img/img_20.png)
3. At the `Import dashboard` modal, click the `SELECT FILE`
![img_21.png](img/img_21.png)
4. Choose the boilerplate dashboard file that included when you cloned the repo: `Finpull/docker/superset/import/dashboard_export_20260101T144643` -> `Select`
![img_22.png](img/img_22.png)
5. After the file is selected, click the `IMPORT` button at the bottom right of the modal
![img_23.png](img/img_23.png)
6. You should see the `Finpull Market Data` dashboard in the dashboard list now `(Note that it may take a while to import depending on your machine performance, if you don't see it try refreshing the page or try adding it manually)`
![img_24.png](img/img_24.png)

### Manual setup dataset
1. Open Superset at `http://localhost:8088`
2. Click `Datasets` on the Navigation panel
![img_25.png](img/img_25.png)
3. At the Dataset panel, click the `+ Dataset` button on the top right
![img_26.png](img/img_26.png)
4. At the `Dataset` panel:
   - Choose `database` dropdown
   - Choose `Clickhouse` database
![img_27.png](img/img_27.png)
5. At the `Dataset` panel:
    - Choose `Schema` dropdown
    - Choose `Schema` provided that you want to use
![img_28.png](img/img_28.png)
6. At the `Dataset` panel:
    - Choose `Table` dropdown
    - Choose `Table` provided that you want to use
![img_29.png](img/img_29.png)
7. Ensure all fields are filled correctly -> Click `Add` button at the bottom right
![img_30.png](img/img_30.png)
8. You should see the new dataset in the dataset list now, click the `Dataset` navigation to refresh if you don't see it
![img_31.png](img/img_31.png)
9. You can now explore the dataset by clicking the dataset name
![img_32.png](img/img_32.png)

### Manual setup charts
1. Open Superset at `http://localhost:8088`
2. Click `Charts` on the Navigation panel
![img_33.png](img/img_33.png)
3. At the Chart panel, click the `+ Chart` button on the top right
![img.png](img/img_34.png)
4. At the `Create a new chart` panel:
   - Choose `Dataset` dropdown
   - Choose the dataset you created earlier
   - Choose `Chart Type` you want to create
   - Click `Create new chart` button at the bottom right
![img_1.png](img/img_35.png)
![img_2.png](img/img_36.png)
   - After create, you will be redirected to the chart edit panel. Next we will create some example charts below
![img_37.png](img/img_37.png)
5. At `TEMPORAL X-AXIS` section:
   - Click the `input` below to add new column
     - A pop-up with three type `(SAVE, SIMPLE AND CUSTOM SQL)` will appear to ask you to choose one and set it up, choose the `CUSTOM SQL` and type in the `ts`. Then click `SAVE` button
![img.png](img/img_38.png)
6. At `METRIC` section:
   - Click the `input` below to add new column
     - A pop-up with three type `(SAVE, SIMPLE AND CUSTOM SQL)` will appear to ask you to choose one and set it up, choose the `SIMPLE`, select `COLUMN: PRICE` and `AGGREGATE: AVG`. Then click `SAVE` button
![img_39.png](img/img_39.png)
7. At the `FILTERS` section:
   - Click the `+` button to add new filter
     - A pop-up will appear to ask you to choose column and condition, choose `COLUMN: SYMBOL`, `OPERATOR: IN` and type in the `value` that you want to filter. Then click `SAVE` button
![img_40.png](img/img_40.png)
8. After all set up, ensure all fields are filled correctly -> Click `RUN` button at the top right to see the chart
![img_41.png](img/img_41.png)
9. You should see the chart rendered now, you can customize more settings as you want and click `UPDATE CHART` button below to run new changes. After that, click the `SAVE` button at the top right to save the chart
![img_42.png](img/img_42.png)


### Manual setup dashboard
1. Open Superset at `http://localhost:8088`
2. Click `Dashboards` on the Navigation panel
![img_43.png](img/img_43.png)
3. At the Dashboard panel, click the `+ Dashboard` button on the top right
![img_44.png](img/img_44.png)
4. At the `Create a new dashboard` modal:
   - You can drag and drop charts from the left panel to the dashboard area to create your dashboard
   - If you dont see any charts, make sure you have created some charts first
   - You can resize and arrange the charts as you want
   - After you are done, click the `SAVE` button at the top right to save the dashboard
   - You can now view your dashboard from the dashboard list
![img_45.png](img/img_45.png)
![img_46.png](img/img_46.png)
![img_47.png](img/img_47.png)
![img_48.png](img/img_48.png)
![img_49.png](img/img_49.png)
![img_50.png](img/img_50.png)


### Access Grafana Dashboard
1. Open Grafana at `http://localhost:3000`
![img.png](img/img_51.png)
2. Login with default credentials (`admin`/`admin`)
![img_1.png](img/img_52.png)
3. At the left panel, click the `Dashboards`
![img_2.png](img/img_53.png)
4. In the Dashboards page, there is a boilerplate dashboard named `Go` for monitoring the application metrics, you can explore in here or create your own dashboards.
![img_3.png](img/img_54.png)
![img_4.png](img/img_55.png)
5. You can also create your own dashboards by clicking the `New` dropdown button on the right panel and selecting `New Dashboard`.
![img_5.png](img/img_56.png)
6. In the new dashboard, click the `Add visualization` button to create your own panels.
![img_6.png](img/img_57.png)
7. A modal of data sources will appear, choose the one that you need (EG: `Prometheus`) and start creating your own queries and visualizations.
![img_7.png](img/img_58.png)
8. After choosing a data sources, you will be redirected to the panel edit page, where you can create your own queries and visualizations.
![img_8.png](img/img_59.png)
9. In the `Queries` section, you can add your own metrics and label for visualization. (EG: `Metrics: go_gc_cycles_automatic_gc_cycles_total`, `Label filter: instance=localhost:9090`)
![img_9.png](img/img_60.png)
10. After adding your queries and run it, you can see the visualization in the panel. After that, click the "Save dashboards" icon on the top right to save your panel.
![img_10.png](img/img_61.png)
11. You can customize your name and description panel (EG: `Title: STU, Description: STU`) and click the `Save` button to save your panel.
![img_11.png](img/img_62.png)
![img_12.png](img/img_63.png)
12. After save the panel, you need to click "Save dashboard" button on the top right to save the whole dashboard.
![img_13.png](img/img_64.png)
![img_14.png](img/img_65.png)
![img_15.png](img/img_66.png)
13. After saving, click the "Dashboards" in the left panel to view your saved dashboard.
![img_16.png](img/img_67.png)
14. You can see your created and you can customize it.
![img_17.png](img/img_68.png)
## Endpoints
- **Health**: `GET /health` - Infrastructure health (ClickHouse + Kafka)
- **Ready**: `GET /ready` - Application readiness (Collector connected)
- **Metrics**: `GET /metrics` - Prometheus metrics

## Metrics

- `finpull_messages_sent_total{backend,symbol}` - Total messages sent
- `finpull_errors_total{type}` - Total errors
- `finpull_last_price{symbol}` - Last recorded price
- `finpull_operation_duration_seconds{operation}` - Operation latency

## Project Structure Best Practices

### What Goes Where?

**`pkg/`** - Infrastructure utilities (reusable across projects):
- Connection managers (DB, Kafka, Redis)
- Configuration loaders
- HTTP servers
- Metrics recorders
- Logging utilities

**`internal/`** - Application-specific logic:
- Domain models and interfaces
- Business logic (use cases)
- Interface adapters (repositories, services)
- Dependency injection setup

**`cmd/`** - Executable entry points:
- Parse flags
- Load config
- Wire dependencies
- Run application
- **That's it. No business logic.**

### Anti-patterns to Avoid

**DO NOT** create connections in repositories  
**DO** inject connections from `pkg/`

**DO NOT** put business logic in `cmd/`  
**DO** keep `cmd/` minimal (3 steps)

**DO NOT** import `internal/` from `pkg/`  
**DO** keep `pkg/` independent and reusable

**DO NOT** use global variables  
**DO** inject dependencies via Wire

## License

MIT

## Contributing

1. Follow Clean Architecture principles
2. Run `make lint` before commit
3. Keep cyclomatic complexity < 15
4. Add tests for new features
5. Update docs

---

**Built with**: Go 1.24 | Wire | ClickHouse | Kafka | Prometheus | Grafana
