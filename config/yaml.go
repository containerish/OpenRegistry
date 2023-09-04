package config

import (
	"crypto/rsa"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/golang-jwt/jwt/v5"
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

	authConfig := viper.GetStringMap("registry.auth")
	if authConfig == nil {
		return nil, fmt.Errorf("missing registry.auth config")
	}

	privKey, pubKey, err := getRSAKeyPairFromViperConfig(authConfig)
	if err != nil {
		return nil, err
	}

	viper.Set("registry.auth.priv_key", privKey)
	viper.Set("registry.auth.pub_key", pubKey)

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("ERR_UNMARSHAL_CONFIG: %w", err)
	}

	setDefaultsForStorageBackend(&cfg)

	githubConfig := cfg.Integrations.GetGithubConfig()
	if githubConfig.Host == "" {
		githubConfig.Host = "0.0.0.0"
	}

	if githubConfig.Port == 0 {
		githubConfig.Port = 5001
	}

	cfg.Integrations.SetGithubConfig(githubConfig)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

const fiveMBInBytes = 1024 * 1024 * 5
const twentyMBInBytes = 1024 * 1024 * 20

func getRSAKeyPairFromViperConfig(authConfig map[string]any) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privKeyPath, ok := authConfig["priv_key"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("invalid type for registry.auth.priv_key")
	}

	pubKeyPath, ok := authConfig["pub_key"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("invalid type for registry.auth.pub_key")
	}

	privKeyBz, err := os.ReadFile(privKeyPath)
	if err != nil {
		return nil, nil, err
	}
	privKey, err := jwt.ParseRSAPrivateKeyFromPEM(privKeyBz)
	if err != nil {
		return nil, nil, err
	}

	pubBz, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return nil, nil, err
	}
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubBz)
	if err != nil {
		return nil, nil, err
	}

	return privKey, pubKey, nil
}

func setDefaultsForStorageBackend(cfg *OpenRegistryConfig) {
	if cfg.DFS.Filebase.Enabled {
		if cfg.DFS.Filebase.ChunkSize == 0 {
			cfg.DFS.Filebase.ChunkSize = twentyMBInBytes
		}

		if cfg.DFS.Filebase.MinChunkSize == 0 {
			cfg.DFS.Filebase.MinChunkSize = fiveMBInBytes
		}
	}

	if cfg.DFS.Storj.Enabled {
		if cfg.DFS.Storj.ChunkSize == 0 {
			cfg.DFS.Storj.ChunkSize = twentyMBInBytes
		}

		if cfg.DFS.Storj.MinChunkSize == 0 {
			cfg.DFS.Storj.MinChunkSize = fiveMBInBytes
		}
	}

	if cfg.StoreConfig.SSLMode == "" {
		cfg.StoreConfig.SSLMode = "disable"
	}
}
