package config

import "github.com/spf13/viper"

func ReadYamlConfig() (*OpenRegistryConfig, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.openregistry")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var registryConfig OpenRegistryConfig
	if err := viper.Unmarshal(&registryConfig); err != nil {
		return nil, err
	}

	return &registryConfig, nil
}
