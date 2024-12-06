# ExchangeBot

## Frontend
- Установить npm, yarn, vite
- cd exchangebot/frontend
- Установка зависимостей `yarn install` 
- Построить проект `yarn build`
- Запустить сервер vite hot reload `yarn dev`
  
При запуске `yarn dev` CONTENT_EMBED=false

## Backend
Для embed.go нужна директория frontend/dist  которая создается при yarn build 

## Docker 
- В первый раз может потребоваться вход  
`docker login`
- Создание docker  образа  
`docker build -t exchange_app .`
- Запуск с arg    
`docker build --build-arg GITHUB_TOKEN=<your_github_token> -t exchange_app .`
- Запуск контейнера Docker: Эта команда запускает контейнер из образа в интерактивном режиме (-it). Флаг --rm удаляет контейнер после его остановки  
`docker run --rm -it sambly/exchange_app`
- Отправить docker в docker-hub  
`docker push sambly/exchange_app:latest`
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







