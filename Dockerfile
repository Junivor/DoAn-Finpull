# Build stage
FROM golang:1.24-bullseye AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://proxy.golang.org
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /finpull ./cmd/app

# Final image
FROM gcr.io/distroless/static:nonroot
# Copy binary
COPY --from=builder /finpull /finpull
EXPOSE 2112
USER nonroot:nonroot
ENTRYPOINT ["/finpull"]
