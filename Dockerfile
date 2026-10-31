# Основной Dockerfile: собирает приложение из исходников.
# Так собирает Amvera и так запускает преподаватель (нужен только Docker): docker build .
#
# Вариант для CI: Dockerfile.artifact — кладёт в образ ГОТОВЫЙ бинарник myapp,
# собранный на предыдущем шаге конвейера (см. .github/workflows/ci-cd.yml).

# --- Стадия 1: сборка из исходников ---
FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o myapp .

# --- Стадия 2: запускаемый образ ---
FROM alpine:3.20
WORKDIR /root/
ENV DATA_DIR=/data
# /data — постоянное хранилище заметок (на Amvera туда монтируется persistenceMount: /data,
# в docker compose — именованный volume), поэтому данные переживают обновление.
RUN mkdir -p /data
VOLUME ["/data"]
EXPOSE 8080
# Docker раз в 10 секунд проверяет /health; статус виден в `docker ps` (healthy / unhealthy)
HEALTHCHECK --interval=10s --timeout=4s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/health || exit 1
COPY --from=builder /app/myapp ./myapp
CMD ["./myapp"]
