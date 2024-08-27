# exchangeApp


# Frontend
установить npm 
добавить yarn 
добавить vite 

yarn create vite
Set the "project name" to "fronted"
Set the "web framework" to "vanilf js"

// установить зависимости  
exchangeBot\frontend\yarn install 
// построить проект
yarn build 
// или запустить сервер с hot    
yarn dev 

# Backend
для файла embed нужна директория frontend/dist  которая создается при yarn build 

# make
// Получение зависимостей
make all
// перед push(для проверки .env)
make lint 

# docker 
// В первый раз может потребоваться вход 
docker login

//Эта команда создает Docker-образ на основе Dockerfile, находящегося в текущей директории.
docker build -t sambly/exchange_app .

// Запуск контейнера Docker: Эта команда запускает контейнер из образа в интерактивном режиме (-it). Флаг --rm удаляет контейнер после его остановки.
docker run --rm -it sambly/exchange_app

// Отправить docker в docker-hub
docker push sambly/exchange_app:latest

# docker-compose
// запуск docker compose c построением. Эта команда запускает службы, определенные в docker-compose.yml, и перед запуском пересобирает образы. -d запуск в фоновом режиме 
docker-compose up --build -d 

// запуск docker compose Эта команда запускает службы, определенные в docker-compose.yml, без пересборки образов. Она использует существующие образы
docker-compose up -d 

// Удаление контейнеров 
docker-compose down 
