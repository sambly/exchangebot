# Этап сборки фронтенда
FROM node:23-alpine AS frontend

WORKDIR /app/frontend

# Копируем только файлы, необходимые для установки зависимостей
COPY ./frontend/package.json ./frontend/yarn.lock ./frontend/vite.config.js ./

# Установка переменных окружения
ARG VITE_GRAFANA_URL="grafana"
ENV VITE_GRAFANA_URL=${VITE_GRAFANA_URL}

# Установка зависимостей и сборка
RUN yarn install
COPY ./frontend ./
RUN yarn build

# Этап сборки Go-приложения
FROM golang:1.24.3-alpine AS builder

RUN apk add --no-cache git
WORKDIR /app

COPY go.mod go.sum ./

# Настройка Git для приватных репозиториев
ARG GITHUB_TOKEN
ENV ENVIRONMENT=docker
RUN git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"

# Установка зависимостей
RUN go mod download

# Копируем фронтенд и исходный код
COPY --from=frontend /app/frontend/dist ./frontend/dist
COPY internal ./internal
COPY cmd ./cmd
COPY embed.go ./
COPY ./configs ./configs
COPY ./config.yaml ./config.yaml

RUN go build -o ./exchangebot ./cmd/cobra

# Финальный образ
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && ln -sf /usr/share/zoneinfo/Europe/Moscow /etc/localtime \
    && echo "Europe/Moscow" > /etc/timezone


WORKDIR /app

COPY --from=builder /app/exchangebot .
COPY --from=builder /app/configs /app/configs
COPY --from=builder /app/config.yaml /app/config.yaml

VOLUME /app/log
EXPOSE 80

CMD ["./exchangebot"]
