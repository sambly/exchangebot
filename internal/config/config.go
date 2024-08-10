package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/joho/godotenv"
)

type Config struct {

	// config app
	ServerName string

	// Web
	Production                    bool
	InProductionOnlyApp           bool
	InProductionWithFrontedNgingx bool
	HttpPortProduction            string
	HostWeb                       string
	// Authentication Web basic
	UsernameAuth string
	PasswordAuth string

	// Exchange
	ApiKey    string
	SecretKey string

	// TLG
	TlgToken string
	TlgUser  string

	// DB
	NameDb     string
	PasswordDb string
	HostDb     string
	PortDb     string
	UserDb     string

	// Log
	DebugLog      bool
	ProductionLog bool

	// GRPC
	GrpcHost string
	GrpcPort string
}

func loadEnv(projectDirName string) error {
	//const projectDirName = "exchangeBot" // change to relevant project name

	projectName := regexp.MustCompile(`^(.*` + projectDirName + `)`)
	currentWorkDirectory, _ := os.Getwd()
	rootPath := projectName.Find([]byte(currentWorkDirectory))
	err := godotenv.Load(string(rootPath) + `/.env`)
	if err != nil {
		return fmt.Errorf("error loading .env file")
	}
	return nil
}

func NewConfig() (*Config, error) {

	//  production определяет как использовать статические файлы fs.Sub(fsys, "dist") os.DirFS("fronted/dist")
	//	inProductionOnlyApp Работа сервера https/http при условии, при условии что только этот сервер запущен
	// 	inProductionWithFrontedNgingx работа на локальном порту, Nginx шлет на этот порт
	// 	inProductionOnlyApp = false  inProductionWithFrontedNgingx = false  локальная разработка для тестов порт 80
	production := false
	inProductionOnlyApp := false
	inProductionWithFrontedNgingx := false

	productionLog := false
	debugLog := false

	var hostDb string
	var hostGrpc string

	if os.Getenv("ENVIRONMENT") == "docker" {
		var exists bool
		hostDb, exists = os.LookupEnv("DB_HOST_Docker")
		if !exists {
			return nil, fmt.Errorf("no .env str DB_HOST_Docker  found")
		}
		hostGrpc, exists = os.LookupEnv("grpc_Host_Docker")
		if !exists {
			return nil, fmt.Errorf("no .env str grpc_Host_Docker  found")
		}

	} else {
		var exists bool
		if err := loadEnv("exchangeBot"); err != nil {
			return nil, err
		}
		hostDb, exists = os.LookupEnv("DB_HOST_Local")
		if !exists {
			return nil, fmt.Errorf("no .env str DB_HOST_Local  found")
		}

		hostGrpc, exists = os.LookupEnv("grpc_Host_Local")
		if !exists {
			return nil, fmt.Errorf("no .env str grpc_Host_Local  found")
		}
	}

	serverName, exists := os.LookupEnv("serverName")
	if !exists {
		return nil, fmt.Errorf("no .env str serverName  found")
	}

	// Web
	productionString, exists := os.LookupEnv("production")
	if !exists {
		return nil, fmt.Errorf("no .env str production found")
	}
	if productionString == "true" {
		production = true
	}
	inProductionOnlyAppString, exists := os.LookupEnv("inProductionOnlyApp")
	if !exists {
		return nil, fmt.Errorf("no .env str inProductionOnlyApp found")
	}
	if inProductionOnlyAppString == "true" {
		inProductionOnlyApp = true
	}
	inProductionWithFrontedNgingxString, exists := os.LookupEnv("inProductionWithFrontedNgingx")
	if !exists {
		return nil, fmt.Errorf("no .env str inProductionWithFrontedNgingx found")
	}
	if inProductionWithFrontedNgingxString == "true" {
		inProductionWithFrontedNgingx = true
	}
	httpPortProduction, exists := os.LookupEnv("httpPortProduction")
	if !exists {
		return nil, fmt.Errorf("no .env str httpPortProduction  found")
	}
	hostWeb, exists := os.LookupEnv("hostWeb")
	if !exists {
		return nil, fmt.Errorf("no .env str hostWeb  found")
	}

	// Exchange
	apiKey, exists := os.LookupEnv("API_KEY")
	if !exists {
		return nil, fmt.Errorf("no .env str API_KEY found")
	}
	secretKey, exists := os.LookupEnv("API_SECRET")
	if !exists {
		return nil, fmt.Errorf("no .env str API_SECRET found")
	}
	// TLG
	tlgToken, exists := os.LookupEnv("TELEGRAM_TOKEN")
	if !exists {
		return nil, fmt.Errorf("no .env str TELEGRAM_TOKEN found")
	}
	tlgUser, exists := os.LookupEnv("TELEGRAM_USER")
	if !exists {
		return nil, fmt.Errorf("no .env str TELEGRAM_USER  found")
	}
	// Authentication
	usernameAuth, exists := os.LookupEnv("usernameAuth")
	if !exists {
		return nil, fmt.Errorf("no .env str usernameAuth  found")
	}
	passwordAuth, exists := os.LookupEnv("passwordAuth")
	if !exists {
		return nil, fmt.Errorf("no .env str passwordAuth  found")
	}
	// DB
	nameDb, exists := os.LookupEnv("DB_NAME")
	if !exists {
		return nil, fmt.Errorf("no .env str DB_NAME found")
	}
	passwordDb, exists := os.LookupEnv("DB_PASSWORD")
	if !exists {
		return nil, fmt.Errorf("no .env str DB_PASSWORD found")
	}
	portDb, exists := os.LookupEnv("DB_PORT")
	if !exists {
		return nil, fmt.Errorf("no .env str DB_PORT found")
	}

	userDb, exists := os.LookupEnv("DB_USER")
	if !exists {
		return nil, fmt.Errorf("no .env str DB_USER found")
	}

	productionLogString, exists := os.LookupEnv("productionLog")
	if !exists {
		return nil, fmt.Errorf("no .env str productionLog found")
	}
	if productionLogString == "true" {
		productionLog = true
	}

	debugLogString, exists := os.LookupEnv("debugLog")
	if !exists {
		return nil, fmt.Errorf("no .env str debugLog found")
	}
	if debugLogString == "true" {
		debugLog = true
	}

	grpcPort, exists := os.LookupEnv("grpc_Port")
	if !exists {
		return nil, fmt.Errorf("no .env str grpc_Port  found")
	}

	c := &Config{

		// config app
		ServerName: serverName,

		// Web
		Production:                    production,
		InProductionOnlyApp:           inProductionOnlyApp,
		InProductionWithFrontedNgingx: inProductionWithFrontedNgingx,
		HttpPortProduction:            httpPortProduction,
		HostWeb:                       hostWeb,
		// Authentication Web basic
		UsernameAuth: usernameAuth,
		PasswordAuth: passwordAuth,

		// Exchange
		ApiKey:    apiKey,
		SecretKey: secretKey,

		// TLG
		TlgToken: tlgToken,
		TlgUser:  tlgUser,

		// DB
		NameDb:     nameDb,
		PasswordDb: passwordDb,
		HostDb:     hostDb,
		PortDb:     portDb,
		UserDb:     userDb,

		// Log
		ProductionLog: productionLog,
		DebugLog:      debugLog,

		// GRPC
		GrpcHost: hostGrpc,
		GrpcPort: grpcPort,
	}
	return c, nil
}
