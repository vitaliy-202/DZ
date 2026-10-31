# Этап сборки
FROM golang:alpine AS builder
WORKDIR /app

# Копируем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь исходный код
COPY . .

# Компилируем приложение под Linux
RUN CGO_ENABLED=0 GOOS=linux go build -o myapp .

# Этап запуска
FROM alpine:latest
WORKDIR /root/

# Копируем скомпилированный бинарник из первого этапа
COPY --from=builder /app/myapp .

EXPOSE 8080
CMD ["./myapp"]