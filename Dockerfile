# Этап сборки фронтенда
FROM node:23-alpine AS frontend

# Рабочая директорая
WORKDIR /app/frontend

# Копирование файлов package.json, yarn.lock для установки зависимостей
COPY ./frontend/package.json ./frontend/yarn.lock ./
# Копирование vite.config.js в рабочую директорию
COPY ./frontend/vite.config.js ./

# # Установка переменных окружения как аргументов сборки
ARG VITE_GRAFANA_URL

# Экспорт аргумента как переменной окружения
ENV VITE_GRAFANA_URL=${VITE_GRAFANA_URL}

# Установить зависимости фронтенда
RUN yarn install

# Копируем остальные файлы фронтенда, за исключением указанных в .dockerignore
COPY ./frontend ./

# Сборка фронтенда
RUN yarn build



# Этап сборки Go-приложения
FROM golang:1.24.3-alpine AS builder

# Установка необходимого для сборки
RUN apk add --no-cache git

# Установка рабочей директории
WORKDIR /app

COPY go.mod go.sum ./

ARG GITHUB_TOKEN
ENV ENVIRONMENT=docker

# Настройка git с использованием переменной GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"

# Установка зависимостей Go
RUN go mod download

# Копирование скомпилированного фронтенда
COPY --from=frontend /app/frontend/dist ./frontend/dist
# Копируем остальные файлы проекта
COPY internal ./internal
COPY cmd ./cmd
COPY embed.go ./
COPY ./configs ./configs
COPY ./config.yaml ./config.yaml

RUN go build -o ./exchangebot ./cmd/cobra

# Финальный образ для запуска
FROM alpine:3.21

# Установка зависимостей для запуска Go сервера
RUN apk add --no-cache ca-certificates

RUN apk add --no-cache ca-certificates tzdata \
    && ln -sf /usr/share/zoneinfo/Europe/Moscow /etc/localtime \
    && echo "Europe/Moscow" > /etc/timezone


WORKDIR /app

COPY --from=builder /app/exchangebot .
COPY --from=builder /app/configs /app/configs
COPY --from=builder /app/config.yaml /app/config.yaml

VOLUME /app/log

EXPOSE 80

#Используем entrypoint для запуска исполняемого файла
CMD ["./exchangebot"]
