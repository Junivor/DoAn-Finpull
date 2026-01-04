# FinPull - Bộ Thu Thập Dữ Liệu Thị Trường Theo Thời Gian Thực

Kiến trúc sạch + Dependency Injection (Wire) để thu thập dữ liệu thị trường theo thời gian thực từ Finnhub và truyền đến Kafka/ClickHouse.

## Kiến Trúc Hệ Thống

![img.png](img/img.png)

## Kiến Trúc

```
pkg/                          # Lớp hạ tầng chia sẻ
├── server/app.go              → Vòng đời ứng dụng (Start/Stop/Health/Ready)
├── clickhouse/client.go       → Quản lý connection pool + khởi tạo schema
├── kafka/client.go            → Quản lý writer + cấu hình batch/compression
├── config/config.go           → Trình tải cấu hình YAML + xác thực
└── metrics/prometheus.go      → Ghi nhận metrics Prometheus

internal/
├── domain/
│   ├── models/trade.go        → Các thực thể nghiệp vụ
│   └── repository/            → Giao diện port (trừu tượng hóa)
│       └── interfaces.go      → MarketStream, Publisher, Storage, Metrics
├── repository/                → Adapters (triển khai giao diện)
│   ├── clickhouse/storage.go  → Adapter lưu trữ ClickHouse
│   └── kafka/publisher.go     → Adapter xuất bản Kafka
├── service/
│   └── finnhub/client.go      → Client Finnhub WebSocket (bên thứ ba)
├── usecase/                   → Logic nghiệp vụ
│   ├── trade_processor.go     → Xử lý & định tuyến giao dịch đến backend
│   └── trade_collector.go     → Thu thập giao dịch từ luồng thị trường
└── di/                        → Dependency injection
    ├── providers.go           → Hàm cung cấp cho Wire
    └── wire.go                → Định nghĩa injector Wire

cmd/app/main.go                → Điểm khởi đầu (3 bước: tải config → wire DI → chạy)
```

## Nguyên Tắc Chính

### 1. **Kiến Trúc Sạch (Clean Architecture)**
- **Lớp Domain**: Logic nghiệp vụ thuần túy, không phụ thuộc bên ngoài
- **Lớp Use case**: Quy tắc nghiệp vụ cụ thể cho ứng dụng
- **Interface adapters**: Chuyển đổi dữ liệu giữa domain và hệ thống bên ngoài
- **Infrastructure**: Frameworks, công cụ, drivers (ClickHouse, Kafka, HTTP)

### 2. **Dependency Injection với Wire**
- Tất cả phụ thuộc được inject qua Wire
- Không có trạng thái toàn cục hoặc singleton
- Dễ dàng kiểm thử và mock

### 3. **Tách Biệt Các Mối Quan Tâm**
- **pkg/**: Thiết lập hạ tầng (kết nối, cấu hình) - CÓ THỂ TÁI SỬ DỤNG
- **internal/**: Logic ứng dụng (domain, usecases, adapters) - CỤ THỂ CHO NGHIỆP VỤ
- **cmd/**: Điểm khởi đầu - TỐI THIỂU (3 bước)

### 4. **Quản Lý Kết Nối**
- Kết nối được tạo **MỘT LẦN** trong `pkg/`
- Repository nhận kết nối, không bao giờ tạo chúng
- Dọn dẹp được xử lý tập trung

## Bắt Đầu Nhanh

### Cài Đặt Công Cụ

```bash
make install-tools
```

Cài đặt:
- Wire (trình tạo mã dependency injection)
- golangci-lint (linter)

### Tạo Mã Wire

```bash
make wire
```

Tạo ra `internal/di/wire_gen.go`.

### Lint

```bash
make lint
```

Áp dụng:
- Độ phức tạp cyclomatic < 15
- Độ phức tạp nhận thức < 15
- Độ dài hàm < 100 dòng
- Xử lý lỗi
- Phong cách code

### Kiểm Thử

```bash
make test
```

## Khởi động dự án cùng hạ tầng với Docker

### Khởi Động Hạ Tầng

```bash
make docker-up
```

Khởi động:
- ClickHouse (`:8123`, `:9000`) UI: `http://localhost:9000`
- Kafka (`:9092`)
- Zookeeper
- Postgres (`:5432`)
- Redis (`:6379`)
- Superset (`:8088`)
- Prometheus (`:9090`)
- Grafana (`:3000`) UI: `http://localhost:3000`

### Tắt dự án và dọn sạch hạ tầng Docker

```bash
make docker-down
```

## Thiết Lập Bổ Sung
### Thiết lập kết nối Superset đến ClickHouse
1. Mở Superset tại `http://localhost:8088`
![img_1.png](img/img_1.png)
2. Đăng nhập với thông tin mặc định (`admin`/`admin`)
![img_2.png](img/img_2.png)
3. Vào `Settings` -> `Database connection`
![img_3.png](img/img_3.png)
4. Vào `Supporting databases`
![img_4.png](img/img_4.png)
5. Chọn `ClickHouse Connect (Superset)`
![img_5.png](img/img_5.png)
6. Chọn `Connect this database with a SQLAlchemy URI string instead`
![img_6.png](img/img_6.png)
7. Sử dụng SQLAlchemy URI: `clickhouse+native://default@clickhouse:9000/finpull` -> `Test Connection` -> `Success` -> `Connect`
![img_6.1.png](img/img_6.1.png)
![img_6.2.png](img/img_6.2.png)

### Import dataset mẫu
1. Chọn `Dataset` trên thanh điều hướng
![img_7.png](img/img_7.png)
2. Nhấn vào `import icon` ở góc trên bên phải cạnh nút `+ Dataset`
![img_8.png](img/img_8.png)
3. Tại modal `Import dataset`, nhấn `SELECT FILE`
![img_9.png](img/img_9.png)
4. Chọn file dataset mẫu đã được bao gồm khi bạn clone repo: `Finpull/docker/superset/import/dataset_export_20260101T144629` -> `Select`
![img_10.png](img/img_10.png)
5. Sau khi file được chọn, nhấn nút `IMPORT` ở góc dưới bên phải của modal
![img_11.png](img/img_11.png)
6. Bạn sẽ thấy dataset `finpull_trades` trong danh sách dataset bây giờ `(Lưu ý rằng có thể mất một chút thời gian để import tùy thuộc vào hiệu suất máy của bạn, nếu bạn không thấy, thử làm mới trang hoặc thử thêm thủ công)`
![img_12.png](img/img_12.png)

### Import biểu đồ mẫu
1. Chọn `Charts` trên thanh điều hướng
![img_13.png](img/img_13.png)
2. Nhấn vào `import icon` ở góc trên bên phải cạnh nút `+ Chart`
![img_14.png](img/img_14.png)
3. Tại modal `Import chart`, nhấn `SELECT FILE`
![img_15.png](img/img_15.png)
4. Chọn file chart mẫu đã được bao gồm khi bạn clone repo: `Finpull/docker/superset/import/chart_export_20260101T144638` -> `Select`
![img_16.png](img/img_16.png)
5. Sau khi file được chọn, nhấn nút `IMPORT` ở góc dưới bên phải của modal
![img_17.png](img/img_17.png)
6. Bạn sẽ thấy nhiều biểu đồ trong danh sách biểu đồ bây giờ `(Lưu ý rằng có thể mất một chút thời gian để import tùy thuộc vào hiệu suất máy của bạn, nếu bạn không thấy, thử làm mới trang hoặc thử thêm thủ công)`
![img_18.png](img/img_18.png)

### Import dashboard mẫu dashboard_export_20260101T144643
1. Chọn `Dashboards` trên thanh điều hướng
![img_19.png](img/img_19.png)
2. Nhấn vào `import icon` ở góc trên bên phải cạnh nút `+ Dashboard`
![img_20.png](img/img_20.png)
3. Tại modal `Import dashboard`, nhấn `SELECT FILE`
![img_21.png](img/img_21.png)
4. Chọn file dashboard mẫu đã được bao gồm khi bạn clone repo: `Finpull/docker/superset/import/dashboard_export_20260101T144643` -> `Select`
![img_22.png](img/img_22.png)
5. Sau khi file được chọn, nhấn nút `IMPORT` ở góc dưới bên phải của modal
![img_23.png](img/img_23.png)
6. Bạn sẽ thấy dashboard `Finpull Market Data` trong danh sách dashboard bây giờ `(Lưu ý rằng có thể mất một chút thời gian để import tùy thuộc vào hiệu suất máy của bạn, nếu bạn không thấy, thử làm mới trang hoặc thử thêm thủ công)`
![img_24.png](img/img_24.png)

### Thiết lập dataset thủ công
1. Mở Superset tại `http://localhost:8088`
2. Nhấn `Datasets` trên thanh điều hướng
![img_25.png](img/img_25.png)
3. Tại panel Dataset, nhấn nút `+ Dataset` ở góc trên bên phải
![img_26.png](img/img_26.png)
4. Tại panel `Dataset`:
   - Chọn dropdown `database`
   - Chọn database `Clickhouse`
![img_27.png](img/img_27.png)
5. Tại panel `Dataset`:
    - Chọn dropdown `Schema`
    - Chọn `Schema` được cung cấp mà bạn muốn sử dụng
![img_28.png](img/img_28.png)
6. Tại panel `Dataset`:
    - Chọn dropdown `Table`
    - Chọn `Table` được cung cấp mà bạn muốn sử dụng
![img_29.png](img/img_29.png)
7. Đảm bảo tất cả các trường được điền chính xác -> Nhấn nút `Add` ở góc dưới bên phải
![img_30.png](img/img_30.png)
8. Bạn sẽ thấy dataset mới trong danh sách dataset bây giờ, nhấn điều hướng `Dataset` để làm mới nếu bạn không thấy
![img_31.png](img/img_31.png)
9. Bạn có thể khám phá dataset bằng cách nhấn vào tên dataset
![img_32.png](img/img_32.png)

### Thiết lập biểu đồ thủ công
1. Mở Superset tại `http://localhost:8088`
2. Nhấn `Charts` trên thanh điều hướng
![img_33.png](img/img_33.png)
3. Tại panel Chart, nhấn nút `+ Chart` ở góc trên bên phải
![img.png](img/img_34.png)
4. Tại panel `Create a new chart`:
   - Chọn dropdown `Dataset`
   - Chọn dataset bạn đã tạo trước đó
   - Chọn `Chart Type` bạn muốn tạo
   - Nhấn nút `Create new chart` ở góc dưới bên phải
![img_1.png](img/img_35.png)
![img_2.png](img/img_36.png)
   - Sau khi tạo, bạn sẽ được chuyển đến panel chỉnh sửa biểu đồ. Tiếp theo chúng ta sẽ tạo một số biểu đồ mẫu bên dưới
![img_37.png](img/img_37.png)
5. Tại phần `TEMPORAL X-AXIS`:
   - Nhấn vào `input` bên dưới để thêm cột mới
     - Một cửa sổ bật lên với ba loại `(SAVE, SIMPLE AND CUSTOM SQL)` sẽ xuất hiện để yêu cầu bạn chọn một và thiết lập, chọn `CUSTOM SQL` và gõ vào `ts`. Sau đó nhấn nút `SAVE`
![img.png](img/img_38.png)
6. Tại phần `METRIC`:
   - Nhấn vào `input` bên dưới để thêm cột mới
     - Một cửa sổ bật lên với ba loại `(SAVE, SIMPLE AND CUSTOM SQL)` sẽ xuất hiện để yêu cầu bạn chọn một và thiết lập, chọn `SIMPLE`, chọn `COLUMN: PRICE` và `AGGREGATE: AVG`. Sau đó nhấn nút `SAVE`
![img_39.png](img/img_39.png)
7. Tại phần `FILTERS`:
   - Nhấn nút `+` để thêm bộ lọc mới
     - Một cửa sổ bật lên sẽ xuất hiện để yêu cầu bạn chọn cột và điều kiện, chọn `COLUMN: SYMBOL`, `OPERATOR: IN` và gõ vào `value` mà bạn muốn lọc. Sau đó nhấn nút `SAVE`
![img_40.png](img/img_40.png)
8. Sau khi thiết lập tất cả, đảm bảo tất cả các trường được điền chính xác -> Nhấn nút `RUN` ở góc trên bên phải để xem biểu đồ
![img_41.png](img/img_41.png)
9. Bạn sẽ thấy biểu đồ được render bây giờ, bạn có thể tùy chỉnh thêm các cài đặt như bạn muốn và nhấn nút `UPDATE CHART` bên dưới để chạy các thay đổi mới. Sau đó, nhấn nút `SAVE` ở góc trên bên phải để lưu biểu đồ
![img_42.png](img/img_42.png)


### Thiết lập dashboard thủ công
1. Mở Superset tại `http://localhost:8088`
2. Nhấn `Dashboards` trên thanh điều hướng
![img_43.png](img/img_43.png)
3. Tại panel Dashboard, nhấn nút `+ Dashboard` ở góc trên bên phải
![img_44.png](img/img_44.png)
4. Tại modal `Create a new dashboard`:
   - Bạn có thể kéo và thả các biểu đồ từ panel bên trái vào khu vực dashboard để tạo dashboard của bạn
   - Nếu bạn không thấy biểu đồ nào, hãy đảm bảo bạn đã tạo một số biểu đồ trước
   - Bạn có thể thay đổi kích thước và sắp xếp các biểu đồ như bạn muốn
   - Sau khi hoàn thành, nhấn nút `SAVE` ở góc trên bên phải để lưu dashboard
   - Bạn có thể xem dashboard của mình từ danh sách dashboard
![img_45.png](img/img_45.png)
![img_46.png](img/img_46.png)
![img_47.png](img/img_47.png)
![img_48.png](img/img_48.png)
![img_49.png](img/img_49.png)
![img_50.png](img/img_50.png)


### Truy cập Grafana Dashboard
1. Mở Grafana tại `http://localhost:3000`
![img.png](img/img_51.png)
2. Đăng nhập với thông tin mặc định (`admin`/`admin`)
![img_1.png](img/img_52.png)
3. Tại panel bên trái, nhấn `Dashboards`
![img_2.png](img/img_53.png)
4. Trong trang Dashboards, có một dashboard mẫu có tên `Go` để giám sát các metrics ứng dụng, bạn có thể khám phá ở đây hoặc tạo dashboard của riêng bạn.
![img_3.png](img/img_54.png)
![img_4.png](img/img_55.png)
5. Bạn cũng có thể tạo dashboard của riêng mình bằng cách nhấn nút dropdown `New` trên panel bên phải và chọn `New Dashboard`.
![img_5.png](img/img_56.png)
6. Trong dashboard mới, nhấn nút `Add visualization` để tạo các panel của riêng bạn.
![img_6.png](img/img_57.png)
7. Một modal của các nguồn dữ liệu sẽ xuất hiện, chọn nguồn bạn cần (VD: `Prometheus`) và bắt đầu tạo các truy vấn và trực quan hóa của riêng bạn.
![img_7.png](img/img_58.png)
8. Sau khi chọn nguồn dữ liệu, bạn sẽ được chuyển đến trang chỉnh sửa panel, nơi bạn có thể tạo các truy vấn và trực quan hóa của riêng mình.
![img_8.png](img/img_59.png)
9. Trong phần `Queries`, bạn có thể thêm các metrics và label của riêng bạn để trực quan hóa. (VD: `Metrics: go_gc_cycles_automatic_gc_cycles_total`, `Label filter: instance=localhost:9090`)
![img_9.png](img/img_60.png)
10. Sau khi thêm các truy vấn của bạn và chạy nó, bạn có thể thấy trực quan hóa trong panel. Sau đó, nhấn biểu tượng "Save dashboards" ở góc trên bên phải để lưu panel của bạn.
![img_10.png](img/img_61.png)
11. Bạn có thể tùy chỉnh tên và mô tả panel của bạn (VD: `Title: STU, Description: STU`) và nhấn nút `Save` để lưu panel của bạn.
![img_11.png](img/img_62.png)
![img_12.png](img/img_63.png)
12. Sau khi lưu panel, bạn cần nhấn nút "Save dashboard" ở góc trên bên phải để lưu toàn bộ dashboard.
![img_13.png](img/img_64.png)
![img_14.png](img/img_65.png)
![img_15.png](img/img_66.png)
13. Sau khi lưu, nhấn "Dashboards" trong panel bên trái để xem dashboard đã lưu của bạn.
![img_16.png](img/img_67.png)
14. Bạn có thể thấy dashboard đã tạo và có thể tùy chỉnh nó.
![img_17.png](img/img_68.png)
## Endpoints
- **Health**: `GET /health` - Sức khỏe hạ tầng (ClickHouse + Kafka)
- **Ready**: `GET /ready` - Sẵn sàng ứng dụng (Collector đã kết nối)
- **Metrics**: `GET /metrics` - Prometheus metrics

## Metrics

- `finpull_messages_sent_total{backend,symbol}` - Tổng số tin nhắn đã gửi
- `finpull_errors_total{type}` - Tổng số lỗi
- `finpull_last_price{symbol}` - Giá được ghi nhận cuối cùng
- `finpull_operation_duration_seconds{operation}` - Độ trễ thao tác

## Các Thực Hành Tốt Về Cấu Trúc Dự Án

### Những thư mục quan trọng

**`pkg/`** - Tiện ích hạ tầng (có thể tái sử dụng qua các dự án):
- Trình quản lý kết nối (DB, Kafka, Redis)
- Trình tải cấu hình
- Máy chủ HTTP
- Trình ghi nhận metrics
- Tiện ích logging

**`internal/`** - Logic cụ thể cho ứng dụng:
- Mô hình domain và giao diện
- Logic nghiệp vụ (use cases)
- Interface adapters (repositories, services)
- Thiết lập dependency injection

**`cmd/`** - Điểm khởi đầu có thể thực thi:
- Phân tích flags
- Tải cấu hình
- Wire dependencies
- Chạy ứng dụng
- **Chỉ vậy thôi. Không có logic nghiệp vụ.**

### Các Anti-pattern Cần Tránh

**KHÔNG** tạo kết nối trong repositories  
**NÊN** inject kết nối từ `pkg/`

**KHÔNG** đặt logic nghiệp vụ trong `cmd/`  
**NÊN** giữ `cmd/` tối thiểu (3 bước)

**KHÔNG** import `internal/` từ `pkg/`  
**NÊN** giữ `pkg/` độc lập và có thể tái sử dụng

**KHÔNG** sử dụng biến toàn cục  
**NÊN** inject dependencies qua Wire

## Giấy Phép

MIT

## Đóng Góp

1. Tuân theo nguyên tắc Kiến trúc Sạch
2. Chạy `make lint` trước khi commit
3. Giữ độ phức tạp cyclomatic < 15
4. Thêm tests cho các tính năng mới
5. Cập nhật tài liệu

---

**Xây dựng với**: Go 1.24 | Wire | ClickHouse | Kafka | Prometheus | Grafana

