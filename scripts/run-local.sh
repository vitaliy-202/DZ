#!/usr/bin/env bash
# Запуск всего из исходников (нужен только Docker): bash scripts/run-local.sh
set -euo pipefail
cd "$(dirname "$0")/.."
docker compose up -d --build
for _ in $(seq 1 40); do
  [ "$(docker inspect -f '{{.State.Health.Status}}' go_web_app 2>/dev/null)" = healthy ] && break; sleep 3
done
bash scripts/smoke.sh http://localhost:8080
echo "Приложение http://localhost:8080 · Grafana http://localhost:3000 · Prometheus http://localhost:9090"
