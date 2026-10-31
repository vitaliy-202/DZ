# DZ — менеджер заметок на Go: контейнер, CI/CD, мониторинг

Веб-приложение на Go (только стандартная библиотека): создание, **редактирование** и удаление заметок.
Развёрнуто на Amvera: https://dz-go-vitaliy-202.msk1.amvera.tech

## Запуск из исходников (для проверяющего)

Нужен только Docker:

```bash
git clone https://github.com/vitaliy-202/DZ.git && cd DZ
bash scripts/run-local.sh
```

| Что | Адрес |
|---|---|
| Приложение | http://localhost:8080 |
| Health check | http://localhost:8080/health |
| Метрики (JSON) | http://localhost:8080/metrics |
| Метрики (формат Prometheus) | http://localhost:8080/metrics/prometheus |
| Grafana (без логина) | http://localhost:3000 |
| Prometheus | http://localhost:9090 |

Без Docker: `go run .` → http://localhost:8080. Тесты: `go test ./...`.

## Процесс сборки и деплоя

`git push` в `main` запускает `.github/workflows/ci-cd.yml`:

1. **Тесты** — gofmt, go vet, go test.
2. **Сборка артефакта** — `scripts/build.sh` собирает статический бинарник `myapp` (+ контрольная сумма); он сохраняется как артефакт запуска (Actions → Artifacts → `go-binary-artifact`).
3. **Упаковка в контейнер** — скачивается *тот же* артефакт, проверяется контрольная сумма, `Dockerfile` (стадия `artifact`) кладёт его в образ. Затем контейнер запускается, проходит smoke-тест (`scripts/smoke.sh`: health, страница, создание/правка/удаление заметки, метрики), пересоздаётся с тем же томом — проверяется, что данные сохранились, — и должен стать `healthy`.
4. **Деплой** — Amvera пересобирает сервис из репозитория по тому же `Dockerfile` и `amvera.yml` и запускает его с постоянным хранилищем `/data`.

## Сохранение данных

Заметки пишутся в `$DATA_DIR/notes.json` (в контейнере `DATA_DIR=/data`). На Amvera `/data` — постоянное хранилище (`persistenceMount` в `amvera.yml`), в docker compose — именованный volume `notes-data`. Запись атомарная (временный файл + rename).

## Метрики

| Требование | Где |
|---|---|
| Посещений в час | `visits_per_hour` (JSON), `app_visits_last_hour` (Prometheus); считаются открытия `/` |
| Health check | `/health`, Docker `HEALTHCHECK`, `up{job="go_app"}` |
| Ресурсы потребляемые | `ram_alloc_mb`, `process_resident_memory_bytes`, `app_container_memory_usage_bytes`, CPU процесса |
| Ресурсы доступные | `ram_sys_mb`, лимиты контейнера (`app_container_memory_limit_bytes`, `app_container_cpu_limit_cores`), ресурсы сервера (node-exporter) |

Дашборд Grafana: `monitoring/grafana/dashboards/dz.json`.

## Файлы

`Dockerfile` — описание контейнера · `docker-compose.yml` — приложение + Prometheus + Grafana + node-exporter · `scripts/build.sh` — сборка · `scripts/smoke.sh` — проверка · `scripts/run-local.sh` — запуск всего · `amvera.yml` — конфигурация Amvera.
