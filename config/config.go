package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/viper"
)

type (
	RegistryConfig struct {
		AuthConfig      AuthConfig   `mapstructure:"auth_config"`
		LogConfig       LogConfig    `mapstructure:"log_config"`
		SkynetConfig    SkynetConfig `mapstructure:"skynet_config"`
		Environment     string       `mapstructure:"environment"`
		DNSAddress      string       `mapstructure:"dns_address"`
		SkynetPortalURL string       `mapstructure:"skynet_portal_url"`
		SigningSecret   string       `mapstructure:"signing_secret"`
		Host            string       `mapstructure:"host"`
		Port            uint         `mapstructure:"port"`
		Debug           bool         `mapstructure:"debug"`
	}

	AuthConfig struct {
		SupportedServices map[string]bool `mapstructure:"supported_services"`
	}

	SkynetConfig struct {
		EndpointPath    string `mapstructure:"endpoint_path"`
		ApiKey          string `mapstructure:"api_key"`
		CustomUserAgent string `mapstructure:"custom_user_agent"`
		CustomCookie    string `mapstructure:"custom_cookie"`
	}

	LogConfig struct {
		Service    string `mapstructure:"service"`
		Endpoint   string `mapstructure:"endpoint"`
		AuthMethod string `mapstructure:"auth_method"`
		Username   string `mapstructure:"username"`
		Password   string `mapstructure:"password"`
	}

	StoreConfig struct {
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Database string `mapstructure:"database"`
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
	}
)

func (r *RegistryConfig) Address() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

func NewStoreConfig() (*StoreConfig, error) {
	return &StoreConfig{
		User:     "postgres",
		Password: "Qwerty@123",
		Database: "open_registry",
		Host:     "0.0.0.0",
		Port:     5432,
	}, nil
}

func (sc *StoreConfig) Endpoint() string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", sc.User, sc.Password, sc.Host, sc.Port, sc.Database)
}

func LoadFromENV() (*RegistryConfig, error) {
	// TODO - Add Support for loading from config
	// loading config from path is not possible right now,
	// since aksah does not support pull from private container repositories
	// viper.AddConfigPath(path)
	// viper.SetConfigName("config")
	// viper.SetConfigType("yaml")

	viper.SetEnvPrefix("OPEN_REGISTRY")
	viper.AutomaticEnv()

	config := RegistryConfig{
		Debug:           viper.GetBool("DEBUG"),
		Environment:     viper.GetString("ENVIRONMENT"),
		Host:            viper.GetString("HOST"),
		Port:            viper.GetUint("PORT"),
		SkynetPortalURL: viper.GetString("SKYNET_PORTAL_URL"),
		SigningSecret:   viper.GetString("SIGNING_SECRET"),
		DNSAddress:      viper.GetString("DNS_ADDRESS"),
		SkynetConfig: SkynetConfig{
			ApiKey:          viper.GetString("SKYNET_API_KEY"),
			CustomUserAgent: fmt.Sprintf("OpenRegistry-%s", viper.GetString("VERSION")),
			CustomCookie:    viper.GetString("SKYNET_CUSTOM_COOKIE"),
		},
		AuthConfig: AuthConfig{
			SupportedServices: make(map[string]bool),
		},
		LogConfig: LogConfig{
			Service:    viper.GetString("LOG_SERVICE_NAME"),
			Endpoint:   viper.GetString("LOG_SERVICE_HOST"),
			AuthMethod: viper.GetString("LOG_SERVICE_AUTH_KIND"),
			Username:   viper.GetString("LOG_SERVICE_USER"),
			Password:   viper.GetString("LOG_SERVICE_PASSWORD"),
		},
	}

	for _, service := range strings.Split(viper.GetString("SUPPORTED_SERVICES"), ",") {
		config.AuthConfig.SupportedServices[service] = true
	}

	if config.SigningSecret == "" {
		fmt.Println("signing secret absent")
		os.Exit(1)
	}

	isProd := config.Environment == "stage" || config.Environment == "production"
	if isProd && config.DNSAddress == "" {
		color.Red("dns address must be set while using stage/prod environments")
		os.Exit(1)
	}

	return &config, nil
}

func (r *RegistryConfig) Endpoint() string {
	switch r.Environment {
	case Dev, Local:
		return fmt.Sprintf("http://%s:%d", r.Host, r.Port)
	case Prod, Stage:
		return fmt.Sprintf("https://%s", r.DNSAddress)
	case CI:
		ciSysAddr := os.Getenv("CI_SYS_ADDR")
		if ciSysAddr == "" {
			log.Fatalln("missing required environment variable: CI_SYS_ADDR")
		}

		return fmt.Sprintf("http://%s", ciSysAddr)
	default:
		return fmt.Sprintf("https://%s:%d", r.Host, r.Port)
	}
}

const (
	Prod  = "production"
	Stage = "stage"
	Dev   = "development"
	Local = "local"
	CI    = "ci"
)
