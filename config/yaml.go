package config

import (
	"os"

	"github.com/fatih/color"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

func ReadYamlConfig() (*OpenRegistryConfig, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.openregistry")

	var registryConfig OpenRegistryConfig
	// OPENREGISTRY_CONFIG env variable takes precedence over everything
	if yamlConfigInEnv := os.Getenv("OPENREGISTRY_CONFIG"); yamlConfigInEnv != "" {
		err := yaml.Unmarshal([]byte(yamlConfigInEnv), &registryConfig)
		if err != nil {
			return nil, err
		}

		if err = registryConfig.Validate(); err != nil {
			return nil, err
		}
		color.Green("read configuration from environment variable")
		return &registryConfig, nil
	}

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	if err := viper.Unmarshal(&registryConfig); err != nil {
		return nil, err
	}

	if err := registryConfig.Validate(); err != nil {
		return nil, err
	}

	return &registryConfig, nil
}
