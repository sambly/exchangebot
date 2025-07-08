# ExchangeBot

## Frontend
- Установить npm, yarn, vite
- cd exchangebot/frontend
- Установка зависимостей `yarn install` 
- Построить проект `yarn build`
- Запустить сервер vite hot reload `yarn dev`
- Включить sourcemap `yarn dev --mode=development`

  
При запуске `yarn dev` CONTENT_EMBED=false

## Backend
- Для embed.go нужна директория frontend/dist  которая создается при yarn build 
- Запуск приложения  `go run ./cmd/cobra`
- Запуск приложения с отлеживанием data race `go run --race ./cmd/cobra` 

## Docker 
- В первый раз может потребоваться вход  
`docker login`
- Создание docker  образа  
`docker build -t exchangebot .`
- Запуск с arg    
`docker build --build-arg GITHUB_TOKEN=<your_github_token> -t exchangebot .`
- Запуск контейнера Docker: Эта команда запускает контейнер из образа в интерактивном режиме (-it). Флаг --rm удаляет контейнер после его остановки  
`docker run --rm -it sambly/exchangebot`
- Отправить docker в docker-hub  
`docker push sambly/exchangebot:latest`
- Просто запустить docker без docker-compose с уже введеными аргументами  
`make build-simple-docker`  
`make run-simple-docker`


## Docker-compose
- Запуск docker compose c построением. Эта команда запускает службы, определенные в docker-compose.yml, и перед запуском пересобирает образы. -d запуск в фоновом режиме  
`docker-compose up --build -d`
- Запуск docker compose Эта команда запускает службы, определенные в docker-compose.yml, без пересборки образов. Она использует существующие образы
`docker-compose up -d` 
- Удаление контейнеров 
`docker-compose down` 

## Cobra 
- Запуск в общем случае  
 `go run ./cmd/cobra`
- Запуск со сборкой (windows)  
 `go build -o exchangebot.exe ./cmd/cobra/main.go`
- Запуск   
 `./exchangebot.exe`
- Глобальная установка (примерно)   
 `go install ./cmd/cobra`
- Применение команд на примере update   
windows (если запусккать просто через run) `go run ./cmd/cobra update --production-log=true`   
docker  `/app # ./exchangebot update --debug-log=true`
- Записать пары в файл pairs.txt   `go run ./cmd/cobra pairs-to-file`

## Pprof 

### Основные команды
- Веб-интерфейс: `http://localhost:6060/debug/pprof/` 
- Профиль в браузере:
  ```
  go tool pprof -http=:8081 http://localhost:6060/debug/pprof/profile?seconds=30
  ```
- Интерактивный режим:
  ```
  go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
  ```

### Интерактивные команды
- `top`: Показать топ функций по времени выполнения.
- `list <FuncName>`: Показать исходный код конкретной функции с аннотациями производительности.
- `web`: Открыть SVG-график в браузере для визуального представления графа вызовов.
- `svg`: Сохранить граф вызовов в формате SVG.
- `png` / `pdf`: Сохранить граф вызовов в формате PNG или PDF.
- `quit`: Выйти из интерактивного режима.
