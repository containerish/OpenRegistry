package config

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/viper"
)

type (
	OpenRegistryConfig struct {
		Registry                *Registry `yaml:"registry" mapstructure:"registry"`
		StoreConfig             *Store    `yaml:"database" mapstructure:"database"`
		AuthConfig              *Auth     `yaml:"auth" mapstructure:"auth"`
		LogConfig               *Log      `yaml:"log_service" mapstructure:"log_service"`
		SkynetConfig            *Skynet   `yaml:"skynet" mapstructure:"skynet"`
		OAuth                   *OAuth    `yaml:"oauth" mapstructure:"oauth"`
		Email                   *Email    `yaml:"email" mapstructure:"email"`
		Environment             string    `yaml:"environment" mapstructure:"environment"`
		WebAppEndpoint          string    `yaml:"web_app_url" mapstructure:"web_app_url"`
		WebAppRedirectURL       string    `yaml:"web_app_redirect_url" mapstructure:"web_app_redirect_url"`
		WebAppErrorRedirectPath string    `yaml:"web_app_error_redirect_path" mapstructure:"web_app_error_redirect_path"`
		Debug                   bool      `yaml:"debug" mapstructure:"debug"`
	}

	Registry struct {
		TLS           TLS      `yaml:"tls" mapstructure:"tls"`
		DNSAddress    string   `yaml:"dns_address" mapstructure:"dns_address"`
		FQDN          string   `yaml:"fqdn" mapstructure:"fqdn"`
		SigningSecret string   `yaml:"jwt_signing_secret" mapstructure:"jwt_signing_secret"`
		Host          string   `yaml:"host" mapstructure:"host"`
		Services      []string `yaml:"services" mapstructure:"services"`
		Port          uint     `yaml:"port" mapstructure:"port"`
	}

	TLS struct {
		PrivateKey string `yaml:"priv_key" mapstructure:"priv_key"`
		PubKey     string `yaml:"pub_key" mapstructure:"pub_key"`
	}

	Auth struct {
		SupportedServices map[string]bool `yaml:"supported_services" mapstructure:"supported_services"`
	}

	Skynet struct {
		SkynetPortalURL string `yaml:"portal_url" mapstructure:"portal_url"`
		EndpointPath    string `yaml:"endpoint_path" mapstructure:"endpoint_path"`
		ApiKey          string `yaml:"api_key" mapstructure:"api_key"`
		CustomUserAgent string `yaml:"custom_user_agent" mapstructure:"custom_user_agent"`
		CustomCookie    string `yaml:"custom_cookie" mapstructure:"custom_cookie"`
	}

	Log struct {
		Service    string `yaml:"name" mapstructure:"name"`
		Endpoint   string `yaml:"endpoint" mapstructure:"endpoint"`
		AuthMethod string `yaml:"auth_method" mapstructure:"auth_method"`
		Username   string `yaml:"username" mapstructure:"username"`
		Password   string `yaml:"password" mapstructure:"password"`
	}

	Store struct {
		Kind     string `yaml:"kind" mapstructure:"kind"`
		User     string `yaml:"username" mapstructure:"username"`
		Host     string `yaml:"host" mapstructure:"host"`
		Password string `yaml:"password" mapstructure:"password"`
		Database string `yaml:"name" mapstructure:"name"`
		Port     int    `yaml:"port" mapstructure:"port"`
	}

	GithubOAuth struct {
		Provider     string `yaml:"provider" mapstructure:"provider"`
		ClientID     string `yaml:"client_id" mapstructure:"client_id"`
		ClientSecret string `yaml:"client_secret" mapstructure:"client_secret"`
	}

	OAuth struct {
		Github GithubOAuth `yaml:"github" mapstructure:"github"`
	}

	Email struct {
		ApiKey                   string `yaml:"api_key" mapstructure:"api_key"`
		SendAs                   string `yaml:"send_as" mapstructure:"send_as"`
		VerifyEmailTemplateId    string `yaml:"verify_template_id" mapstructure:"verify_template_id"`
		ForgotPasswordTemplateId string `yaml:"forgot_password_template_id" mapstructure:"forgot_password_template_id"`
		WelcomeEmailTemplateId   string `yaml:"welcome_template_id" mapstructure:"welcome_template_id"`
		Enabled                  bool   `yaml:"enabled" mapstructure:"enabled"`
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
