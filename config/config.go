package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type (
	RegistryConfig struct {
		Debug               bool         `mapstructure:"debug"`
		Environment         string       `mapstructure:"environment"`
		Host                string       `mapstructure:"host"`
		Port                uint         `mapstructure:"port"`
		SkynetPortalURL     string       `mapstructure:"skynet_portal_url"`
		SigningSecret       string       `mapstructure:"signing_secret"`
		SkynetConfig        SkynetConfig `mapstructure:"skynet_config"`
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
	// err := viper.ReadInConfig()
	// if err != nil {
	// 	return nil, err
	// }

	config := RegistryConfig{
		Debug:           viper.GetBool("DEBUG"),
		Environment:     viper.GetString("ENVIRONMENT"),
		Host:            viper.GetString("HOST"),
		Port:            viper.GetUint("PORT"),
		SkynetPortalURL: viper.GetString("SKYNET_PORTAL_URL"),
		SigningSecret:   viper.GetString("SIGNING_SECRET"),
		SkynetConfig:    SkynetConfig{},
	}

	if config.SigningSecret == "" {
		fmt.Println("signing secret absent")
		os.Exit(1)
	}

	return &config, nil
}
