#!/usr/bin/env bash
# Проверка запущенного приложения: bash scripts/smoke.sh [адрес]   (по умолчанию http://localhost:8080)
# SMOKE_WRITE=0 — не создавать заметки (для проверки боевого сервера)
set -euo pipefail
BASE="${1:-http://localhost:8080}"; BASE="${BASE%/}"; WRITE="${SMOKE_WRITE:-1}"
fail() { echo "❌ $*" >&2; exit 1; }
pass() { echo "✅ $*"; }

for _ in $(seq 1 30); do curl -fsS -o /dev/null "$BASE/health" 2>/dev/null && break; sleep 1; done
curl -fsS "$BASE/health"  | grep -q healthy        || fail "/health"
pass "/health"
curl -fsS "$BASE/"        | grep -q 'action="/add"' || fail "главная страница"
pass "главная страница"
curl -fsS "$BASE/metrics" | grep -q visits_per_hour || fail "/metrics (JSON)"
pass "/metrics (JSON)"
curl -fsS "$BASE/metrics/prometheus" | grep -q '^app_visits_last_hour' || fail "/metrics/prometheus"
pass "/metrics/prometheus"

if [ "$WRITE" = "1" ]; then
  curl -fsS -o /dev/null -d 'title=smoke&content=one' "$BASE/add"        || fail "добавление"
  curl -fsS "$BASE/" | grep -q 'value="smoke"'                            || fail "заметка не появилась"
  id="$(curl -fsS "$BASE/" | grep -B1 'value="smoke"' | sed -n 's/.*name="id" value="\([0-9]*\)".*/\1/p' | head -1)"
  [ -n "$id" ] || fail "не нашли id заметки"
  curl -fsS -o /dev/null -d "id=$id&title=smoke&content=edited" "$BASE/edit" || fail "редактирование"
  curl -fsS "$BASE/" | grep -q 'value="edited"'                           || fail "правка не видна"
  curl -fsS -o /dev/null -d "id=$id" "$BASE/delete"                       || fail "удаление"
  pass "создание, редактирование и удаление заметки #$id"
fi
echo "🎉 Smoke-тест пройден: $BASE"
