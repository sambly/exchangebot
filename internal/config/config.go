package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {

	// config app
	ServerName string

	// exchange or grpc
	BuildTarget string

	// Web
	Production  bool
	ProxyServer bool
	ProxyPort   string
	HostWeb     string

	ContentEmbed bool

	// Authentication Web basic
	UsernameAuth string
	PasswordAuth string

	// Exchange
	APIKey    string
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

func (c Config) String() string {
	var sb strings.Builder

	sb.WriteString("Config:\n")
	sb.WriteString(fmt.Sprintf("  ServerName:    %s\n", c.ServerName))
	sb.WriteString(fmt.Sprintf("  BuildTarget:   %s\n", c.BuildTarget))
	sb.WriteString(fmt.Sprintf("  Production:    %v\n", c.Production))
	sb.WriteString(fmt.Sprintf("  ProxyServer:   %v\n", c.ProxyServer))
	sb.WriteString(fmt.Sprintf("  ProxyPort:     %s\n", c.ProxyPort))
	sb.WriteString(fmt.Sprintf("  HostWeb:       %s\n", c.HostWeb))
	sb.WriteString(fmt.Sprintf("  ContentEmbed:  %v\n", c.ContentEmbed))
	sb.WriteString(fmt.Sprintf("  UsernameAuth:  %s\n", c.UsernameAuth))
	sb.WriteString(fmt.Sprintf("  PasswordAuth:  %s\n", c.PasswordAuth))
	sb.WriteString(fmt.Sprintf("  APIKey:        %s\n", c.APIKey))
	sb.WriteString(fmt.Sprintf("  SecretKey:     %s\n", c.SecretKey))
	sb.WriteString(fmt.Sprintf("  TlgToken:      %s\n", c.TlgToken))
	sb.WriteString(fmt.Sprintf("  TlgUser:       %s\n", c.TlgUser))
	sb.WriteString(fmt.Sprintf("  NameDb:        %s\n", c.NameDb))
	sb.WriteString(fmt.Sprintf("  PasswordDb:    %s\n", c.PasswordDb))
	sb.WriteString(fmt.Sprintf("  HostDb:        %s\n", c.HostDb))
	sb.WriteString(fmt.Sprintf("  PortDb:        %s\n", c.PortDb))
	sb.WriteString(fmt.Sprintf("  UserDb:        %s\n", c.UserDb))
	sb.WriteString(fmt.Sprintf("  DebugLog:      %v\n", c.DebugLog))
	sb.WriteString(fmt.Sprintf("  ProductionLog: %v\n", c.ProductionLog))
	sb.WriteString(fmt.Sprintf("  GrpcHost:      %s\n", c.GrpcHost))
	sb.WriteString(fmt.Sprintf("  GrpcPort:      %s\n", c.GrpcPort))

	return sb.String()
}

func loadEnv(projectDirName string) error {
	//const projectDirName = "exchangebot" // change to relevant project name

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
		if err := loadEnv("exchangebot"); err != nil {
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
		APIKey:    apiKey,
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

func NewConfigV2() (*Config, error) {

	// Загружаем значения из Viper
	cfg := &Config{
		ServerName: viper.GetString("server-name"),

		BuildTarget: viper.GetString("build-target"),

		Production:   viper.GetBool("production"),
		ProxyServer:  viper.GetBool("proxy-server"),
		ProxyPort:    viper.GetString("proxy-port"),
		HostWeb:      viper.GetString("host-web"),
		ContentEmbed: viper.GetBool("content-embed"),
		UsernameAuth: viper.GetString("username-auth"),
		PasswordAuth: viper.GetString("password-auth"),

		APIKey:    viper.GetString("api-key-binance"),
		SecretKey: viper.GetString(`api-secret-binance`),

		TlgToken: viper.GetString("telegram-token"),
		TlgUser:  viper.GetString("telegram-user"),

		NameDb:     viper.GetString("db-name"),
		PasswordDb: viper.GetString("db-password"),
		HostDb:     "",
		PortDb:     viper.GetString("db-port"),
		UserDb:     viper.GetString("db-user"),

		DebugLog:      viper.GetBool("debug-log"),
		ProductionLog: viper.GetBool("production-log"),

		GrpcHost: "",
		GrpcPort: viper.GetString("grpc-port"),
	}

	if os.Getenv("ENVIRONMENT") == "docker" {
		cfg.HostDb = viper.GetString("db-host-docker")
		cfg.GrpcHost = viper.GetString("grpc-host-docker")
	} else {
		cfg.HostDb = viper.GetString("db-host-local")
		cfg.GrpcHost = viper.GetString("grpc-host-local")
	}

	// Проверка обязательных параметров

	// if cfg.APIKey == "" || cfg.SecretKey == "" {
	// 	log.Fatal("APIKey and SecretKey are required")
	// }

	return cfg, nil
}
