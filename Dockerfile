# Этап сборки
FROM golang:1.21-alpine3.18 AS builder

# Установка необходимого для сборки
RUN apk add --no-cache git

# Установка рабочей директории
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN go build -o exchangebot .

# Этап сборки фронтенда
FROM node:14-alpine AS frontend

# Установка yarn и дополнительных зависимостей
RUN npm install -g yarn vite

# Устанавливаем рабочую директорию в контейнере
WORKDIR /app/frontend

# Копирование файлов package.json, yarn.lock для установки зависимостей
COPY ./frontend/package.json ./frontend/yarn.lock ./

# Копирование vite.config.js в рабочую директорию
COPY ./frontend/vite.config.js ./
# Установить зависимости фронтенда
RUN yarn install

# Копируем остальные файлы фронтенда, за исключением указанных в .dockerignore
COPY ./frontend/ ./

# Сборка фронтенда
RUN yarn build

# Финальный образ для запуска
FROM alpine:3.18

# Установка зависимостей для запуска Go сервера
RUN apk add --no-cache ca-certificates

# Устанавливаем рабочую директорию в контейнере
WORKDIR /app

# Копировать скомпилированное Go приложение из builder этапа
COPY --from=builder /app/exchangebot .

# Копировать статические файлы фронтенда из frontend этапа
COPY --from=frontend /app/frontend/dist ./dist

# Указать порт, если требуется
EXPOSE 80

# Команда для запуска вашего приложения
CMD ["./exchangebot"]