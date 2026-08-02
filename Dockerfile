# build
FROM golang:1.22-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/assistant ./cmd/server

# runtime
FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata wget \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone
COPY --from=builder /out/assistant /app/assistant
COPY configs/config.yaml /app/configs/config.yaml
RUN mkdir -p /app/workspace /app/screenshots /app/logs /app/temp
ENV SERVER_HOST=0.0.0.0
ENV SERVER_PORT=8080
ENV DESKTOP_WORKSPACE=/app/workspace
ENV LLM_USE_MOCK=true
EXPOSE 8080
ENTRYPOINT ["/app/assistant", "-config", "/app/configs/config.yaml", "-mode", "http"]
