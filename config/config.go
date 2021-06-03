package config

import (
	"fmt"
	"log"
	"os"
	"os/user"

	"github.com/spf13/viper"
)

type (
	RegistryConfig struct {
		Debug               bool         `yaml:"debug"`
		Host                string       `yaml:"host"`
		Port                uint         `yaml:"port"`
		SkynetPortalURL     string       `yaml:"skynet_portal_url"`
		SkynetLinkResolvers []string     `yaml:"skynet_link_resolvers"`
		SkynetStorePath     string       `yaml:"skynet_store_path"`
		TLSCertPath         string       `yaml:"tls_cert_path"`
		TLSKeyPath          string       `yaml:"tls_key_path"`
		SkynetConfig        SkynetConfig `yaml:"skynet_config"`
	}

	SkynetConfig struct {
		EndpointPath    string `yaml:"endpoint_path"`
		ApiKey          string `yaml:"api_key"`
		CustomUserAgent string `yaml:"custom_user_agent"`
	}
)

func (r *RegistryConfig) Address() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

func Load(path string) (*RegistryConfig, error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("parachute")
	viper.SetConfigType("yaml")

	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		setDefaults()
	}

	var config RegistryConfig
	err = viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func setDefaults() {

	user, err := user.Current()
	if err != nil {
		log.Fatalln(err.Error())
	}

	defaultStorePath := user.HomeDir + "/.parachute"
	defaultLinkResolverPath := defaultStorePath + "/links"

	os.MkdirAll(defaultLinkResolverPath, os.ModePerm)

	viper.SetDefault("debug", true)
	viper.SetDefault("host", "0.0.0.0")
	viper.SetDefault("port", "5000")
	viper.SetDefault("tls_key_path", "./certs/key.pem")
	viper.SetDefault("tls_cert_path", "./certs/cert.pem")
	viper.SetDefault("skynet_store_path", defaultStorePath)
	viper.SetDefault("skynet_link_resolvers", defaultLinkResolverPath)
	viper.SetDefault("skynet_portal_url", "https://siasky.net")

	skynetConfig := SkynetConfig{
		EndpointPath:    "",
		ApiKey:          "",
		CustomUserAgent: "",
	}

	viper.SetDefault("skynet_config", skynetConfig)
}
