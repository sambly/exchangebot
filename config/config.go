package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/joho/godotenv"
)

type Config struct {

	// Web
	Production                    bool
	InProductionOnlyApp           bool
	InProductionWithFrontedNgingx bool
	HttpPortProduction            string
	HostWeb                       string

	// Exchange
	ApiKey    string
	SecretKey string
	// TLG
	TlgToken string
	TlgUser  string
	// Authentication Web basic
	UsernameAuth string
	PasswordAuth string
	// DB
	UserNameDb string
	PasswordDb string
	NameDb     string
	HostNameDb string
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

	if err := loadEnv("exchangeBot"); err != nil {
		return nil, err
	}

	// Web
	productionString, exists := os.LookupEnv("production")
	if exists {
		return nil, fmt.Errorf("no .env str production found")
	}
	if productionString == "true" {
		production = true
	}
	inProductionOnlyAppString, exists := os.LookupEnv("inProductionOnlyApp")
	if exists {
		return nil, fmt.Errorf("no .env str inProductionOnlyApp found")
	}
	if inProductionOnlyAppString == "true" {
		inProductionOnlyApp = true
	}
	inProductionWithFrontedNgingxString, exists := os.LookupEnv("inProductionWithFrontedNgingx")
	if exists {
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
	userNameDb, exists := os.LookupEnv("userNameDb")
	if !exists {
		return nil, fmt.Errorf("no .env str userNameDb  found")
	}
	passwordDb, exists := os.LookupEnv("passwordDb")
	if !exists {
		return nil, fmt.Errorf("no .env str passwordDb  found")
	}
	nameDb, exists := os.LookupEnv("nameDb")
	if !exists {
		return nil, fmt.Errorf("no .env str nameDb  found")
	}
	hostNameDb, exists := os.LookupEnv("hostNameDb")
	if !exists {
		return nil, fmt.Errorf("no .env str hostNameDb  found")
	}

	c := &Config{

		Production:                    production,
		InProductionOnlyApp:           inProductionOnlyApp,
		InProductionWithFrontedNgingx: inProductionWithFrontedNgingx,
		HttpPortProduction:            httpPortProduction,
		HostWeb:                       hostWeb,

		ApiKey:    apiKey,
		SecretKey: secretKey,

		TlgToken: tlgToken,
		TlgUser:  tlgUser,

		UsernameAuth: usernameAuth,
		PasswordAuth: passwordAuth,

		UserNameDb: userNameDb,
		PasswordDb: passwordDb,
		NameDb:     nameDb,
		HostNameDb: hostNameDb,
	}
	return c, nil
}
