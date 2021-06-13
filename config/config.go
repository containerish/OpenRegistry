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
		Debug               bool         `mapstructure:"debug"`
		Host                string       `mapstructure:"host"`
		Port                uint         `mapstructure:"port"`
		SkynetPortalURL     string       `mapstructure:"skynet_portal_url"`
		SkynetLinkResolvers []string     `mapstructure:"skynet_link_resolvers"`
		SkynetStorePath     string       `mapstructure:"skynet_store_path"`
		TLSCertPath         string       `mapstructure:"tls_cert_path"`
		TLSKeyPath          string       `mapstructure:"tls_key_path"`
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
