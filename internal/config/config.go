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
	Production  bool
	ProxyServer bool
	ProxyPort   string
	HostWeb     string
	// Authentication Web basic
	UsernameAuth string
	PasswordAuth string

	ContentEmbed bool

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
	//	runProductionServer Работа сервера https/http при условии, при условии что только этот сервер запущен
	// 	runProductionServerProxy работа на локальном порту
	// 	not production  локальная разработка для тестов порт 80
	production := false
	proxyServer := false
	contentEmbed := false
	productionLog := false
	debugLog := false

	var hostDb string
	var hostGrpc string

	if os.Getenv("ENVIRONMENT") == "docker" {
		var exists bool
		hostDb, exists = os.LookupEnv("DB_HOST_DOCKER")
		if !exists {
			return nil, fmt.Errorf("no .env str DB_HOST_DOCKER  found")
		}
		hostGrpc, exists = os.LookupEnv("GRPC_HOST_DOCKER")
		if !exists {
			return nil, fmt.Errorf("no .env str GRPC_HOST_DOCKER  found")
		}

	} else {
		var exists bool
		if err := loadEnv("exchangeBot"); err != nil {
			return nil, err
		}
		hostDb, exists = os.LookupEnv("DB_HOST_LOCAL")
		if !exists {
			return nil, fmt.Errorf("no .env str DB_HOST_LOCAL  found")
		}

		hostGrpc, exists = os.LookupEnv("GRPC_HOST_LOCAL")
		if !exists {
			return nil, fmt.Errorf("no .env str GRPC_HOST_LOCAL  found")
		}
	}

	serverName, exists := os.LookupEnv("SERVERNAME")
	if !exists {
		return nil, fmt.Errorf("no .env str SERVERNAME  found")
	}

	// Web
	productionString, exists := os.LookupEnv("PRODUCTION")
	if !exists {
		return nil, fmt.Errorf("no .env str PRODUCTION found")
	}
	if productionString == "true" {
		production = true
	}

	proxyServerString, exists := os.LookupEnv("PROXY_SERVER")
	if !exists {
		return nil, fmt.Errorf("no .env str PROXY_SERVER found")
	}
	if proxyServerString == "true" {
		proxyServer = true
	}

	contentEmbedString, exists := os.LookupEnv("CONTENT_EMBED")
	if !exists {
		return nil, fmt.Errorf("no .env str CONTENT_EMBED found")
	}
	if contentEmbedString == "true" {
		contentEmbed = true
	}

	proxyPort, exists := os.LookupEnv("PROXY_PORT")
	if !exists {
		return nil, fmt.Errorf("no .env str PROXY_PORT  found")
	}

	hostWeb, exists := os.LookupEnv("HOST_WEB")
	if !exists {
		return nil, fmt.Errorf("no .env str HOST_WEB  found")
	}

	// Authentication
	usernameAuth, exists := os.LookupEnv("USERNAME_AUTH")
	if !exists {
		return nil, fmt.Errorf("no .env str USERNAME_AUTH  found")
	}

	passwordAuth, exists := os.LookupEnv("PASSWORD_AUTH")
	if !exists {
		return nil, fmt.Errorf("no .env str PASSWORD_AUTH  found")
	}

	// Exchange
	apiKey, exists := os.LookupEnv("API_KEY_BINANCE")
	if !exists {
		return nil, fmt.Errorf("no .env str API_KEY_BINANCE found")
	}
	secretKey, exists := os.LookupEnv("API_SECRET_BINANCE")
	if !exists {
		return nil, fmt.Errorf("no .env str API_SECRET_BINANCE found")
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

	// LOG
	productionLogString, exists := os.LookupEnv("PRODUCTION_LOG")
	if !exists {
		return nil, fmt.Errorf("no .env str PRODUCTION_LOG found")
	}
	if productionLogString == "true" {
		productionLog = true
	}

	debugLogString, exists := os.LookupEnv("DEBUG_LOG")
	if !exists {
		return nil, fmt.Errorf("no .env str DEBUG_LOG found")
	}
	if debugLogString == "true" {
		debugLog = true
	}

	grpcPort, exists := os.LookupEnv("GRPC_PORT")
	if !exists {
		return nil, fmt.Errorf("no .env str GRPC_PORT  found")
	}

	c := &Config{

		// config app
		ServerName: serverName,

		// Web
		Production:  production,
		ProxyServer: proxyServer,
		ProxyPort:   proxyPort,
		HostWeb:     hostWeb,

		ContentEmbed: contentEmbed,
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
