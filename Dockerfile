# build
FROM golang:1.22-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go mod tidy \
 && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/assistant ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/mcp-demo ./cmd/mcp-demo

# runtime
FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata wget \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone
COPY --from=builder /out/assistant /app/assistant
COPY --from=builder /out/mcp-demo /app/mcp-demo
COPY configs/config.yaml /app/configs/config.yaml
COPY docs/dev-ops/mysql/sql /app/docs/dev-ops/mysql/sql
RUN mkdir -p /app/workspace /app/screenshots /app/logs /app/temp \
 && chmod +x /app/assistant /app/mcp-demo
ENV SERVER_HOST=0.0.0.0
ENV SERVER_PORT=8080
ENV DESKTOP_WORKSPACE=/app/workspace
ENV DB_TYPE=mysql
ENV MYSQL_HOST=mysql
ENV MYSQL_PORT=3306
ENV MYSQL_DATABASE=ai_desktop_assistant
ENV MYSQL_USER=root
ENV MYSQL_PASSWORD=123456
ENV LLM_USE_MOCK=true
ENV MCP_ENABLED=true
EXPOSE 8080
ENTRYPOINT ["/app/assistant", "-config", "/app/configs/config.yaml", "-mode", "http"]
