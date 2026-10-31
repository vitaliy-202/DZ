#!/usr/bin/env bash
# Сборка АРТЕФАКТА: ./myapp (статический бинарник для Linux). Запуск: bash scripts/build.sh
set -euo pipefail
cd "$(dirname "$0")/.."
CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH:-amd64}" go build -trimpath -ldflags="-s -w" -o myapp .
sha256sum myapp > myapp.sha256
ls -lh myapp && echo "OK: артефакт готов — myapp"
