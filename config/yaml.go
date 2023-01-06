package config

import (
	"fmt"
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

	var cfg OpenRegistryConfig
	// OPENREGISTRY_CONFIG env variable takes precedence over everything
	if yamlConfigInEnv := os.Getenv("OPENREGISTRY_CONFIG"); yamlConfigInEnv != "" {
		err := yaml.Unmarshal([]byte(yamlConfigInEnv), &cfg)
		if err != nil {
			return nil, err
		}

		if err = cfg.Validate(); err != nil {
			return nil, err
		}
		color.Green("read configuration from environment variable")
		return &cfg, nil
	}

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("ERR_READ_IN_CONFIG: %w", err)
	}

	// just a hack for enum typed Environment
	env := strings.ToUpper(viper.GetString("environment"))
	viper.Set("environment", environmentFromString(env))

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("ERR_UNMARSHAL_CONFIG: %w", err)
	}

	if cfg.DFS.Filebase.Enabled && cfg.DFS.Filebase.ChunkSize == 0 {
		cfg.DFS.Filebase.ChunkSize = 1024 * 1024 * 20
	}

	if cfg.DFS.Storj.Enabled && cfg.DFS.Storj.ChunkSize == 0 {
		cfg.DFS.Storj.ChunkSize = 1024 * 1024 * 20
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
