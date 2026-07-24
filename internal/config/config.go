package config

import (
	"fmt"
	"os"
	"reflect"

	"github.com/spf13/viper"
)

type App struct {
	ServerName    string `mapstructure:"server-name" yaml:"server-name"`
	ExchangeType  string `mapstructure:"exchange-type" yaml:"exchange-type"`
	PairsFromFile bool   `mapstructure:"pairs-from-file" yaml:"pairs-from-file"`
	PprofEnable   bool   `mapstructure:"pprof-enable" yaml:"pprof-enable"`
}

type Web struct {
	ListenPort   string `mapstructure:"listen-port" yaml:"listen-port"`
	Host         string `mapstructure:"host" yaml:"host"`
	UseTLC       bool   `mapstructure:"use-tls" yaml:"use-tls"`
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
	Enable             bool   `mapstructure:"enable" yaml:"enable"`
	UseProxy           bool   `mapstructure:"use-proxy" yaml:"use-proxy"`
	ProxyURL           string `mapstructure:"proxy-url" yaml:"proxy-url"`

	// UseCustomAPI переключает базовый URL Bot API с api.telegram.org на
	// CustomAPIURL - неофициальный адрес (прокладка), которая сама уже стучится
	// в Telegram. В отличие от UseProxy (туннелирование через HTTP/SOCKS-прокси
	// до настоящего api.telegram.org) здесь меняется сам хост назначения запроса.
	UseCustomAPI bool   `mapstructure:"use-custom-api" yaml:"use-custom-api"`
	CustomAPIURL string `mapstructure:"custom-api-url" yaml:"custom-api-url"`

	// WebhookEnable переключает бота с long-polling на webhook. Публичный URL
	// вебхука собирается как WebhookURL+WebhookPath и должен быть уже проброшен
	// внешним reverse-proxy на веб-сервер приложения.
	WebhookEnable bool   `mapstructure:"webhook-enable" yaml:"webhook-enable"`
	WebhookURL    string `mapstructure:"webhook-url" yaml:"webhook-url"`
	WebhookPath   string `mapstructure:"webhook-path" yaml:"webhook-path"`
	WebhookSecret string `mapstructure:"webhook-secret" yaml:"webhook-secret"`
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

type Tracer struct {
	EnableOpenTelemetry  bool   `mapstructure:"enable-open-telemetry" yaml:"enable-open-telemetry"`
	OtelExporterEndpoint string `mapstructure:"otel-exporter-otlp-endpoint" yaml:"otel-exporter-otlp-endpoint"`
	OtelServiceName      string `mapstructure:"otel-service-name" yaml:"otel-service-name"`
}

type Notification struct {
	Enable bool `mapstructure:"enable" yaml:"enable"`
}

// Depth - по каким парам держать L2-стакан (internal/depth.AssetsDepth).
//
// По умолчанию список пуст: depth-поток заметно тяжелее тикеров/трейдов
// (полный стакан, частые диффы), и включать его сразу на все торгуемые пары
// (их бывает несколько сотен) значило бы молча взвинтить нагрузку на
// exchange_service и объём памяти под стаканы. AllPairs включает его для всех
// пар из settings.Pairs - как и в internal/strategy/anomaly.Config.
type Depth struct {
	AllPairs bool     `mapstructure:"all-pairs" yaml:"all-pairs"`
	Pairs    []string `mapstructure:"pairs" yaml:"pairs"`
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
	Depth        `mapstructure:"depth" yaml:"depth"`
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

func NewConfig() (*Config, error) {

	cfg := &Config{}
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshal viper config file: %v", err)
	}

	if os.Getenv("ENVIRONMENT") == "docker" {
		cfg.Database.Host = cfg.Database.HostDocker
		cfg.GRPC.Host = cfg.GRPC.HostDocker
	} else {
		cfg.Database.Host = cfg.Database.HostLocal
		cfg.GRPC.Host = cfg.GRPC.HostLocal
	}

	return cfg, nil
}
