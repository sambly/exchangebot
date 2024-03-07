package config

import (
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/joho/godotenv"
)

type Config struct {
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
	UserNameDb         string
	PasswordDb         string
	NameDb             string
	HostNameDb         string
	HttpPortProduction string
}

func loadEnv() {

}

func NewConfig() (*Config, error) {

	const projectDirName = "exchangeBot" // change to relevant project name

	projectName := regexp.MustCompile(`^(.*` + projectDirName + `)`)
	currentWorkDirectory, _ := os.Getwd()
	rootPath := projectName.Find([]byte(currentWorkDirectory))
	err := godotenv.Load(string(rootPath) + `/.env`)
	if err != nil {
		log.Fatalf("Error loading .env file")
		return nil, fmt.Errorf("error loading .env file")
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
	// httpPort
	httpPortProduction, exists := os.LookupEnv("httpPortProduction")
	if !exists {
		return nil, fmt.Errorf("no .env str httpPortProduction  found")
	}

	c := &Config{

		ApiKey:    apiKey,
		SecretKey: secretKey,

		TlgToken: tlgToken,
		TlgUser:  tlgUser,

		UsernameAuth: usernameAuth,
		PasswordAuth: passwordAuth,

		UserNameDb:         userNameDb,
		PasswordDb:         passwordDb,
		NameDb:             nameDb,
		HostNameDb:         hostNameDb,
		HttpPortProduction: httpPortProduction,
	}
	return c, nil
}
