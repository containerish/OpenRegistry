package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/viper"
)

type (
	RegistryConfig struct {
		AuthConfig      AuthConfig   `mapstructure:"auth_config"`
		SkynetConfig    SkynetConfig `mapstructure:"skynet_config"`
		Host            string       `mapstructure:"host"`
		SkynetPortalURL string       `mapstructure:"skynet_portal_url"`
		SigningSecret   string       `mapstructure:"signing_secret"`
		Environment     string       `mapstructure:"environment"`
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
	}
)

func (r *RegistryConfig) Address() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
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
		SkynetConfig:    SkynetConfig{},
		AuthConfig: AuthConfig{
			SupportedServices: make(map[string]bool),
		},
	}

	for _, service := range strings.Split(viper.GetString("SUPPORTED_SERVICES"), ",") {
		config.AuthConfig.SupportedServices[service] = true
	}

	color.Red("supported auth backends: %v", config.AuthConfig)

	if config.SigningSecret == "" {
		fmt.Println("signing secret absent")
		os.Exit(1)
	}

	return &config, nil
}
