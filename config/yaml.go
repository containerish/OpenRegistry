package config

import (
	"os"
	"strings"

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

	// just a hack for enum typed Environment
	env := strings.ToUpper(viper.GetString("environment"))
	viper.Set("environment", environmentFromString(env))

	if err := viper.Unmarshal(&registryConfig); err != nil {
		return nil, err
	}

	if registryConfig.DFS.S3Any != nil && registryConfig.DFS.S3Any.ChunkSize == 0 {
		registryConfig.DFS.S3Any.ChunkSize = 1024 * 1024 * 20
	}

	if err := registryConfig.Validate(); err != nil {
		return nil, err
	}

	return &registryConfig, nil
}
