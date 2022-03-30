package config

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/viper"
)

type (
	OpenRegistryConfig struct {
		Registry          *Registry `mapstructure:"registry"`
		StoreConfig       *Store    `mapstructure:"database"`
		AuthConfig        *Auth     `mapstructure:"auth"`
		LogConfig         *Log      `mapstructure:"log_service"`
		SkynetConfig      *Skynet   `mapstructure:"skynet"`
		OAuth             *OAuth    `mapstructure:"oauth"`
		Environment       string    `mapstructure:"environment"`
		WebAppEndpoint    string    `mapstructure:"web_app_url"`
		WebAppRedirectURL string    `mapstructure:"web_app_redirect_url"`
		Debug             bool      `mapstructure:"debug"`
		Email             *Email    `mapstructure:"email"`
	}

	Registry struct {
		DNSAddress    string   `mapstructure:"dns_address"`
		SigningSecret string   `mapstructure:"jwt_signing_secret"`
		Host          string   `mapstructure:"host"`
		Services      []string `mapstructure:"services"`
		Port          uint     `mapstructure:"port"`
	}

	Auth struct {
		SupportedServices map[string]bool `mapstructure:"supported_services"`
	}

	Skynet struct {
		SkynetPortalURL string `mapstructure:"portal_url"`
		EndpointPath    string `mapstructure:"endpoint_path"`
		ApiKey          string `mapstructure:"api_key"`
		CustomUserAgent string `mapstructure:"custom_user_agent"`
		CustomCookie    string `mapstructure:"custom_cookie"`
	}

	Log struct {
		Service    string `mapstructure:"name"`
		Endpoint   string `mapstructure:"endpoint"`
		AuthMethod string `mapstructure:"auth_method"`
		Username   string `mapstructure:"username"`
		Password   string `mapstructure:"password"`
	}

	Store struct {
		Kind     string `mapstructure:"kind"`
		User     string `mapstructure:"username"`
		Host     string `mapstructure:"host"`
		Password string `mapstructure:"password"`
		Database string `mapstructure:"name"`
		Port     int    `mapstructure:"port"`
	}

	GithubOAuth struct {
		Provider     string `mapstructure:"provider"`
		ClientID     string `mapstructure:"client_id"`
		ClientSecret string `mapstructure:"client_secret"`
	}

	OAuth struct {
		Github GithubOAuth `mapstructure:"github"`
	}

	Email struct {
		ApiKey               string `mapstructure:"api_key"`
		SendAs               string `mapstructure:"send_as"`
		VerifyEmailTemplate  string `mapstructure:"verify_template_id"`
		WelcomeEmailTemplate string `mapstructure:"welcome_template_id"`
		Enabled              bool   `mapstructure:"enabled"`
	}
)

func (r *Registry) Address() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

func NewStoreConfig() (*Store, error) {
	viper.SetEnvPrefix("OPEN_REGISTRY")
	viper.AutomaticEnv()

	storeConfig := &Store{
		User:     viper.GetString("DB_USER"),
		Password: viper.GetString("DB_PASSWORD"),
		Database: viper.GetString("DB_NAME"),
		Host:     viper.GetString("DB_HOST"),
		Port:     viper.GetInt("DB_PORT"),
	}

	return storeConfig, nil
}

func (sc *Store) Endpoint() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?pool_max_conns=1000&sslmode=disable",
		sc.User, sc.Password, sc.Host, sc.Port, sc.Database)
}

func (oc *OpenRegistryConfig) Endpoint() string {
	switch oc.Environment {
	case Local:
		return fmt.Sprintf("http://%s:%d", oc.Registry.Host, oc.Registry.Port)
	case Prod, Stage:
		return fmt.Sprintf("https://%s", oc.Registry.DNSAddress)
	case CI:
		ciSysAddr := os.Getenv("CI_SYS_ADDR")
		if ciSysAddr == "" {
			log.Fatalln("missing required environment variable: CI_SYS_ADDR")
		}

		return fmt.Sprintf("http://%s", ciSysAddr)
	default:
		return fmt.Sprintf("https://%s:%d", oc.Registry.Host, oc.Registry.Port)
	}
}

const (
	Prod  = "production"
	Stage = "stage"
	Local = "local"
	CI    = "ci"
)
