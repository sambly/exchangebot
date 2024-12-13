package config

import (
	"fmt"
	"os"
	"reflect"

	"github.com/spf13/viper"
)

type App struct {
	// Config app
	ServerName    string `mapstructure:"server-name" yaml:"server-name"`
	BuildTarget   string `mapstructure:"build-target" yaml:"build-target"`
	ExchangeType  string `mapstructure:"exchange-type" yaml:"exchange-type"`
	PairsFromFile bool   `mapstructure:"pairs-from-file" yaml:"pairs-from-file"`
}

type Web struct {
	Production   bool   `mapstructure:"production" yaml:"production"`
	ProxyServer  bool   `mapstructure:"proxy-server" yaml:"proxy-server"`
	ProxyPort    string `mapstructure:"proxy-port" yaml:"proxy-port"`
	Host         string `mapstructure:"host" yaml:"host"`
	ContentEmbed bool   `mapstructure:"content-embed" yaml:"content-embed"`
	UsernameAuth string `mapstructure:"username-auth" yaml:"username-auth"`
	PasswordAuth string `mapstructure:"password-auth" yaml:"password-auth"`
}
type Exchange struct {
	APIKey    string `mapstructure:"api-key" yaml:"api-key"`
	SecretKey string `mapstructure:"secret-key" yaml:"secret-key"`
}
type Telegram struct {
	Token              string `mapstructure:"token" yaml:"token"`
	User               string `mapstructure:"user" yaml:"user"`
	NotificationEnable bool   `mapstructure:"notification-enable" yaml:"notification-enable"`
}
type Database struct {
	Type       string `mapstructure:"type" yaml:"type"`
	Name       string `mapstructure:"name" yaml:"name"`
	Password   string `mapstructure:"password" yaml:"password"`
	Host       string `mapstructure:"host" yaml:"host"`
	HostDocker string `mapstructure:"host-docker" yaml:"host-docker"`
	HostLocal  string `mapstructure:"host-local" yaml:"host-local"`
	Port       string `mapstructure:"port" yaml:"port"`
	User       string `mapstructure:"user" yaml:"user"`
}
type Log struct {
	Debug      bool `mapstructure:"debug" yaml:"debug"`
	Production bool `mapstructure:"production" yaml:"production"`
}
type GRPC struct {
	Host       string `mapstructure:"host" yaml:"host"`
	HostDocker string `mapstructure:"host-docker" yaml:"host-docker"`
	HostLocal  string `mapstructure:"host-local" yaml:"host-local"`
	Port       string `mapstructure:"port" yaml:"port"`
}

// TODO
type Tracer struct {
	OtelExporterEndpoint string `mapstructure:"otel-exporter-otlp-endpoint" yaml:"otel-exporter-otlp-endpoint"`
	OtelServiceName      string `mapstructure:"otel-service-name" yaml:"otel-service-name"`
}

type Notification struct {
	Enable bool `mapstructure:"enable" yaml:"enable"`
}

type Config struct {
	App          `mapstructure:"app" yaml:"app"`
	Web          `mapstructure:"web" yaml:"web"`
	Exchange     `mapstructure:"exchange" yaml:"exchange"`
	Telegram     `mapstructure:"tlg" yaml:"tlg"`
	Database     `mapstructure:"db" yaml:"db"`
	Log          `mapstructure:"log" yaml:"log"`
	GRPC         `mapstructure:"grpc" yaml:"grpc"`
	Tracer       `mapstructure:"tracer" yaml:"tracer"`
	Notification `mapstructure:"notification" yaml:"notification"`
}

func PrintConfig(v interface{}, indent string) {
	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = typ.Elem()
	}

	if val.Kind() != reflect.Struct {
		fmt.Printf("%s%v\n", indent, val)
		return
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		value := val.Field(i)

		fieldName := field.Name
		yamlTag := field.Tag.Get("yaml")
		if yamlTag != "" {
			fieldName = yamlTag
		}

		fmt.Printf("%s%s: ", indent, fieldName)

		if value.Kind() == reflect.Struct {
			fmt.Println()
			PrintConfig(value.Interface(), indent+"  ")
		} else {
			fmt.Printf("%v\n", value.Interface())
		}
	}
}

func NewConfigV3() (*Config, error) {

	cfg := &Config{}
	if err := viper.Unmarshal(&cfg); err != nil {
		// TODO
		return nil, fmt.Errorf("%v", "hui")
	}

	if os.Getenv("ENVIRONMENT") == "docker" {
		cfg.Database.Host = cfg.Database.HostDocker
		cfg.GRPC.Host = cfg.GRPC.HostDocker
	} else {
		cfg.Database.Host = cfg.Database.HostLocal
		cfg.GRPC.Host = cfg.GRPC.HostLocal
	}

	// // Проверка обязательных параметров
	// if cfg.Production && cfg.HostWeb == "" {
	// 	return nil, fmt.Errorf("HostWeb are required ")
	// }
	// if cfg.ProxyServer && cfg.ProxyPort == "" {
	// 	return nil, fmt.Errorf("ProxyPort are required ")
	// }
	// if cfg.APIKey == "" || cfg.SecretKey == "" {
	// 	return nil, fmt.Errorf("APIKey and SecretKey are required")
	// }
	// if cfg.TlgToken == "" || cfg.TlgUser == "" {
	// 	return nil, fmt.Errorf("TlgToken and TlgUser are required")
	// }

	// fmt.Println("↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓")
	// printSettings(viper.AllSettings(), 0)
	// fmt.Println("↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑")

	return cfg, nil

}

func printSettings(settings map[string]interface{}, indent int) {
	for key, value := range settings {
		// Отступы для форматирования
		indentation := ""
		for i := 0; i < indent; i++ {
			indentation += "  "
		}

		// Выводим ключ и значение
		switch v := value.(type) {
		case map[string]interface{}:
			fmt.Printf("%s%s:\n", indentation, key)
			printSettings(v, indent+1)
		default:
			fmt.Printf("%s%s: %v\n", indentation, key, v)
		}
	}
}
