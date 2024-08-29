# Этап сборки фронтенда
FROM node:22-alpine AS frontend

# Рабочая директорая
WORKDIR /app/frontend

# Копирование файлов package.json, yarn.lock для установки зависимостей
COPY ./frontend/package.json ./frontend/yarn.lock ./
# Копирование vite.config.js в рабочую директорию
COPY ./frontend/vite.config.js ./

# Установить зависимости фронтенда
RUN yarn install

# Копируем остальные файлы фронтенда, за исключением указанных в .dockerignore
COPY ./frontend ./

# Сборка фронтенда
RUN yarn build



# Этап сборки Go-приложения
FROM golang:1.22.5-alpine AS builder

# Установка необходимого для сборки
RUN apk add --no-cache git

# Установка рабочей директории
WORKDIR /app

COPY go.mod go.sum ./

# Установка переменных окружения как аргументов сборки
ARG GITHUB_TOKEN
ARG ENVIRONMENT
ARG BUILD_TARGET=exchange

# Установка переменных окружения
ENV GOPRIVATE=github.com/sambly
ENV GITHUB_TOKEN=${GITHUB_TOKEN}
ENV ENVIRONMENT=${ENVIRONMENT}

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

RUN go build -o ./cmd/exchange/exchangebot ./cmd/${BUILD_TARGET}



# Финальный образ для запуска
FROM alpine:3.18

# Установка зависимостей для запуска Go сервера
RUN apk add --no-cache ca-certificates

RUN apk add --no-cache ca-certificates tzdata \
    && ln -sf /usr/share/zoneinfo/Europe/Moscow /etc/localtime \
    && echo "Europe/Moscow" > /etc/timezone

WORKDIR /app/cmd/exchange

COPY --from=builder /app/cmd/exchange/exchangebot .
COPY --from=builder /app/configs /app/configs

VOLUME /app/log

EXPOSE 80

#Используем entrypoint для запуска исполняемого файла
CMD ["./exchangebot"]
