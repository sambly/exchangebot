package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	// config app
	ServerName string `yaml:"server-name" mapstructure:"server-name"`

	// cobra or ncli
	BuildTarget string `yaml:"build-target" mapstructure:"build-target"`

	// exchange or grpc
	ExchangeType string `yaml:"exchange-type" mapstructure:"exchange-type"`

	// Чтение пар из файла
	PairsFromFile bool `yaml:"pairs-from-file" mapstructure:"pairs-from-file"`

	// Web
	Production  bool   `yaml:"production" mapstructure:"production"`
	ProxyServer bool   `yaml:"proxy-server" mapstructure:"proxy-server"`
	ProxyPort   string `yaml:"proxy-port" mapstructure:"proxy-port"`
	HostWeb     string `yaml:"host-web" mapstructure:"host-web"`

	ContentEmbed bool `yaml:"content-embed" mapstructure:"content-embed"`

	// Authentication Web basic
	UsernameAuth string `yaml:"username-auth" mapstructure:"username-auth"`
	PasswordAuth string `yaml:"password-auth" mapstructure:"password-auth"`

	// Exchange
	APIKey    string `yaml:"api-key" mapstructure:"api-key"`
	SecretKey string `yaml:"secret-key" mapstructure:"secret-key"`

	// TLG
	TlgToken string `yaml:"tlg-token" mapstructure:"tlg-token"`
	TlgUser  string `yaml:"tlg-user" mapstructure:"tlg-user"`

	// DB
	NameDb     string `yaml:"name-db" mapstructure:"name-db"`
	PasswordDb string `yaml:"password-db" mapstructure:"password-db"`
	HostDb     string `yaml:"host-db" mapstructure:"host-db"`
	PortDb     string `yaml:"port-db" mapstructure:"port-db"`
	UserDb     string `yaml:"user-db" mapstructure:"user-db"`

	// Log
	DebugLog      bool `yaml:"debug-log" mapstructure:"debug-log"`
	ProductionLog bool `yaml:"production-log" mapstructure:"production-log"`

	// GRPC
	GrpcHost string `yaml:"grpc-host" mapstructure:"grpc-host"`
	GrpcPort string `yaml:"grpc-port" mapstructure:"grpc-port"`
}

func (c Config) String() string {
	var sb strings.Builder

	sb.WriteString("Config:\n")
	sb.WriteString(fmt.Sprintf("  ServerName:    %s\n", c.ServerName))
	sb.WriteString(fmt.Sprintf("  BuildTarget:   %s\n", c.BuildTarget))
	sb.WriteString(fmt.Sprintf("  ExchangeType:  %s\n", c.ExchangeType))
	sb.WriteString(fmt.Sprintf("  PairsFromFile: %v\n", c.PairsFromFile))
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
	sb.WriteString("  ---------------------------------------------------")

	return sb.String()
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
	pairsFromFile := false

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

		if err := godotenv.Load(".env"); err != nil {
			return nil, fmt.Errorf("error loading .env file")
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

	serverName, exists := os.LookupEnv("SERVER_NAME")
	if !exists {
		return nil, fmt.Errorf("no .env str SERVER_NAME  found")
	}

	// Web
	productionString, exists := os.LookupEnv("PRODUCTION")
	if !exists {
		return nil, fmt.Errorf("no .env str PRODUCTION found")
	}
	if productionString == "true" {
		production = true
	}

	// Web
	pairsFromFileString, exists := os.LookupEnv("PAIRS_FROM_FILE")
	if !exists {
		return nil, fmt.Errorf("no .env str PAIRS_FROM_FILE found")
	}
	if pairsFromFileString == "true" {
		pairsFromFile = true
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

	exchangeType, exists := os.LookupEnv("EXCHANGE_TYPE")
	if !exists {
		return nil, fmt.Errorf("no .env str EXCHANGE_TYPE  found")
	}

	c := &Config{

		// config app
		ServerName: serverName,

		ExchangeType: exchangeType,

		PairsFromFile: pairsFromFile,

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

func NewConfigV3() (*Config, error) {

	// Загружаем значения из Viper
	cfg := &Config{
		ServerName: viper.GetString("server-name"),

		BuildTarget: viper.GetString("build-target"),

		ExchangeType: viper.GetString("exchange-type"),

		PairsFromFile: viper.GetBool("pairs-from-file"),

		Production:   viper.GetBool("production"),
		ProxyServer:  viper.GetBool("proxy-server"),
		ProxyPort:    viper.GetString("proxy-port"),
		HostWeb:      viper.GetString("host-web"),
		ContentEmbed: viper.GetBool("content-embed"),
		UsernameAuth: viper.GetString("username-auth"),
		PasswordAuth: viper.GetString("password-auth"),

		APIKey:    viper.GetString("api-key-binance"),
		SecretKey: viper.GetString("api-secret-binance"),

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

	viper.Set("server-name", cfg.ServerName)
	viper.Set("build-target", cfg.BuildTarget)
	viper.Set("exchange-type", cfg.ExchangeType)
	viper.Set("pairs-from-file", cfg.PairsFromFile)
	viper.Set("production", cfg.Production)
	viper.Set("proxy-server", cfg.ProxyServer)
	viper.Set("proxy-port", cfg.ProxyPort)
	viper.Set("host-web", cfg.HostWeb)
	viper.Set("content-embed", cfg.ContentEmbed)
	viper.Set("username-auth", cfg.UsernameAuth)
	viper.Set("password-auth", cfg.PasswordAuth)
	viper.Set("api-key-binance", cfg.APIKey)
	viper.Set("api-secret-binance", cfg.SecretKey)
	viper.Set("telegram-token", cfg.TlgToken)
	viper.Set("telegram-user", cfg.TlgUser)
	viper.Set("db-name", cfg.NameDb)
	viper.Set("db-password", cfg.PasswordDb)
	viper.Set("db-host-docker", cfg.HostDb)
	viper.Set("db-host-local", cfg.HostDb)
	viper.Set("db-port", cfg.PortDb)
	viper.Set("db-user", cfg.UserDb)
	viper.Set("debug-log", cfg.DebugLog)
	viper.Set("production-log", cfg.ProductionLog)
	viper.Set("grpc-port", cfg.GrpcPort)
	viper.Set("grpc-host-docker", cfg.GrpcHost)
	viper.Set("grpc-host-local", cfg.GrpcHost)

	// Проверка обязательных параметров
	if cfg.Production && cfg.HostWeb == "" {
		return nil, fmt.Errorf("HostWeb are required ")
	}
	if cfg.ProxyServer && cfg.ProxyPort == "" {
		return nil, fmt.Errorf("ProxyPort are required ")
	}
	if cfg.APIKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("APIKey and SecretKey are required")
	}
	if cfg.TlgToken == "" || cfg.TlgUser == "" {
		return nil, fmt.Errorf("TlgToken and TlgUser are required")
	}
	return cfg, nil

}
