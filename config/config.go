package config

import (
	"crypto/rsa"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	"github.com/hashicorp/go-multierror"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

type (
	OpenRegistryConfig struct {
		SkynetConfig   Skynet         `yaml:"skynet" mapstructure:"skynet" validate:"-"`
		WebAppConfig   WebAppConfig   `yaml:"web_app" mapstructure:"web_app"`
		OAuth          OAuth          `yaml:"oauth" mapstructure:"oauth" validate:"-"`
		StoreConfig    Store          `yaml:"database" mapstructure:"database" validate:"required"`
		LogConfig      Log            `yaml:"log_service" mapstructure:"log_service"`
		Email          Email          `yaml:"email" mapstructure:"email" validate:"-"`
		Integrations   Integrations   `yaml:"integrations" mapstructure:"integrations"`
		Registry       Registry       `yaml:"registry" mapstructure:"registry" validate:"required"`
		DFS            DFS            `yaml:"dfs" mapstructure:"dfs"`
		WebAuthnConfig WebAuthnConfig `yaml:"web_authn_config" mapstructure:"web_authn_config"`
		Environment    Environment    `yaml:"environment" mapstructure:"environment" validate:"required"`
		Debug          bool           `yaml:"debug" mapstructure:"debug"`
	}

	WebAppConfig struct {
		RedirectURL       string   `yaml:"redirect_url" mapstructure:"redirect_url" validate:"required"`
		ErrorRedirectPath string   `yaml:"error_redirect_path" mapstructure:"error_redirect_path"`
		CallbackURL       string   `yaml:"callback_url" mapstructure:"callback_url"`
		AllowedEndpoints  []string `yaml:"endpoints" mapstructure:"endpoints" validate:"required"`
	}

	DFS struct {
		Filebase S3CompatibleDFS `yaml:"filebase" mapstructure:"filebase"`
		Storj    Storj           `yaml:"storj" mapstructure:"storj"`
	}

	Storj struct {
		Type             string `yaml:"type" mapstructure:"type"`
		AccessGrantToken string `yaml:"access_grant_token" mapstructure:"access_grant_token"`
		LinkShareService string `yaml:"link_share_service" mapstructure:"link_share_service"`
		AccessKey        string `yaml:"access_key" mapstructure:"access_key"`
		SecretKey        string `yaml:"secret_key" mapstructure:"secret_key"`
		Endpoint         string `yaml:"endpoint" mapstructure:"endpoint"`
		BucketName       string `yaml:"bucket_name" mapstructure:"bucket_name"`
		DFSLinkResolver  string `yaml:"dfs_link_resolver" mapstructure:"dfs_link_resolver"`
		ChunkSize        int    `yaml:"chunk_size" mapstructure:"chunk_size"`
		MinChunkSize     uint64 `yaml:"min_chunk_size" mapstructure:"min_chunk_size"`
		Enabled          bool   `yaml:"enabled" mapstructure:"enabled"`
	}

	S3CompatibleDFS struct {
		AccessKey       string `yaml:"access_key" mapstructure:"access_key"`
		SecretKey       string `yaml:"secret_key" mapstructure:"secret_key"`
		Endpoint        string `yaml:"endpoint" mapstructure:"endpoint"`
		BucketName      string `yaml:"bucket_name" mapstructure:"bucket_name"`
		DFSLinkResolver string `yaml:"dfs_link_resolver" mapstructure:"dfs_link_resolver"`
		ChunkSize       int    `yaml:"chunk_size" mapstructure:"chunk_size"`
		MinChunkSize    uint64 `yaml:"min_chunk_size" mapstructure:"min_chunk_size"`
		Enabled         bool   `yaml:"enabled" mapstructure:"enabled"`
	}

	// just so that we can retrieve values easily
	Integrations []*Integration

	Registry struct {
		DNSAddress string   `yaml:"dns_address" mapstructure:"dns_address" validate:"required"`
		FQDN       string   `yaml:"fqdn" mapstructure:"fqdn" validate:"required"`
		Host       string   `yaml:"host" mapstructure:"host" validate:"required"`
		TLS        TLS      `yaml:"tls" mapstructure:"tls" validate:"-"`
		Auth       Auth     `yaml:"auth" mapstructure:"auth" validate:"required"`
		Services   []string `yaml:"services" mapstructure:"services" validate:"-"`
		Port       uint     `yaml:"port" mapstructure:"port" validate:"required"`
	}

	TLS struct {
		PrivateKey string `yaml:"priv_key" mapstructure:"priv_key"`
		PubKey     string `yaml:"pub_key" mapstructure:"pub_key"`
		Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`
	}

	Skynet struct {
		SkynetPortalURL string `yaml:"portal_url" mapstructure:"portal_url" validate:"-"`
		EndpointPath    string `yaml:"endpoint_path" mapstructure:"endpoint_path"`
		ApiKey          string `yaml:"api_key" mapstructure:"api_key"`
		CustomUserAgent string `yaml:"custom_user_agent" mapstructure:"custom_user_agent"`
	}

	Log struct {
		Service    string `yaml:"name" mapstructure:"name"`
		Endpoint   string `yaml:"endpoint" mapstructure:"endpoint"`
		AuthMethod string `yaml:"auth_method" mapstructure:"auth_method"`
		Username   string `yaml:"username" mapstructure:"username"`
		Password   string `yaml:"password" mapstructure:"password"`
		Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`
	}

	Store struct {
		Kind     string `yaml:"kind" mapstructure:"kind" validate:"required"`
		User     string `yaml:"username" mapstructure:"username" validate:"required"`
		Host     string `yaml:"host" mapstructure:"host" validate:"required"`
		Password string `yaml:"password" mapstructure:"password" validate:"required"`
		Database string `yaml:"name" mapstructure:"name" validate:"required"`
		Port     int    `yaml:"port" mapstructure:"port" validate:"required"`
	}

	GithubOAuth struct {
		ClientID     string `yaml:"client_id" mapstructure:"client_id" validate:"required"`
		ClientSecret string `yaml:"client_secret" mapstructure:"client_secret" validate:"required"`
	}

	OAuth struct {
		Github GithubOAuth `yaml:"github" mapstructure:"github"`
	}

	Email struct {
		ApiKey                string `yaml:"api_key" mapstructure:"api_key" validate:"required"`
		SendAs                string `yaml:"send_as" mapstructure:"send_as" validate:"required"`
		VerifyEmailTemplateId string `yaml:"verify_template_id" mapstructure:"verify_template_id" validate:"required"`
		//nolint
		ForgotPasswordTemplateId string `yaml:"forgot_password_template_id" mapstructure:"forgot_password_template_id" validate:"required"`
		WelcomeEmailTemplateId   string `yaml:"welcome_template_id" mapstructure:"welcome_template_id" validate:"required"`
		Enabled                  bool   `yaml:"enabled" mapstructure:"enabled"`
	}

	WebAuthnConfig struct {
		RPDisplayName string        `yaml:"rp_display_name" mapstructure:"rp_display_name"`
		RPID          string        `yaml:"rp_id" mapstructure:"rp_id"`
		RPIcon        string        `yaml:"rp_icon" mapstructure:"rp_icon"`
		RPOrigin      string        `yaml:"rp_origin" mapstructure:"rp_origin"`
		RPOrigins     []string      `yaml:"rp_origins" mapstructure:"rp_origins"`
		Enabled       bool          `yaml:"enabled" mapstructure:"enabled"`
		Timeout       time.Duration `yaml:"timeout" mapstructure:"timeout"`
	}
	Integration struct {
		Name                  string `yaml:"name" mapstructure:"name"`
		ClientSecret          string `yaml:"client_secret" mapstructure:"client_secret"`
		ClientID              string `yaml:"client_id" mapstructure:"client_id"`
		PublicLink            string `yaml:"public_link" mapstructure:"public_link"`
		PrivateKeyPem         string `yaml:"private_key_pem" mapstructure:"private_key_pem"`
		AppInstallRedirectURL string `yaml:"app_install_redirect_url" mapstructure:"app_install_redirect_url"`
		WebhookSecret         string `yaml:"webhook_secret" mapstructure:"webhook_secret"`
		Host                  string `yaml:"host" mapstructure:"host"`
		AppID                 int64  `yaml:"app_id" mapstructure:"app_id"`
		Port                  int    `yaml:"port" mapstructure:"port"`
		Enabled               bool   `yaml:"enabled" mapstructure:"enabled"`
	}

	Auth struct {
		JWTSigningPrivateKey *rsa.PrivateKey `yaml:"priv_key" mapstructure:"priv_key"`
		JWTSigningPubKey     *rsa.PublicKey  `yaml:"pub_key" mapstructure:"pub_key"`
		Enabled              bool            `yaml:"enabled" mapstructure:"enabled"`
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

func (oc *OpenRegistryConfig) Validate() error {
	if oc == nil {
		return fmt.Errorf("invalid config, cannot be nil")
	}
	v := validator.New()

	english := en.New()
	uni := ut.New(english, english)
	trans, ok := uni.GetTranslator("en")
	if !ok {
		return fmt.Errorf("translation not available for the given language")
	}
	if err := enTranslations.RegisterDefaultTranslations(v, trans); err != nil {
		return err
	}

	var e error
	e = multierror.Append(e, translateError(v.Struct(oc), trans))

	merr := e.(*multierror.Error)
	if merr.ErrorOrNil() != nil {
		return merr
	}

	return nil
}

func translateError(err error, trans ut.Translator) error {
	if err != nil {
		var translatedErr error
		validatorErrs, ok := err.(validator.ValidationErrors)
		if !ok {
			return err
		}
		for _, e := range validatorErrs {
			translatedErr = multierror.Append(translatedErr, fmt.Errorf(e.Translate(trans)))
		}

		return translatedErr
	}

	return nil
}

func (sc *Store) Endpoint() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?pool_max_conns=1000&sslmode=disable",
		sc.User, sc.Password, sc.Host, sc.Port, sc.Database,
	)
}

func (oc *OpenRegistryConfig) Endpoint() string {
	switch oc.Environment {
	case Local:
		if oc.Registry.TLS.Enabled {
			return fmt.Sprintf("https://%s:%d", oc.Registry.Host, oc.Registry.Port)
		}
		return fmt.Sprintf("http://%s:%d", oc.Registry.Host, oc.Registry.Port)
	case Production, Staging:
		return fmt.Sprintf("https://%s", oc.Registry.DNSAddress)
	case CI:
		ciSysAddr := os.Getenv("CI_SYS_ADDR")
		if ciSysAddr == "" {
			log.Fatalln("missing required environment variable: CI_SYS_ADDR")
		}

		if oc.Registry.TLS.Enabled {
			return fmt.Sprintf("https://%s", ciSysAddr)
		}

		return fmt.Sprintf("http://%s", ciSysAddr)
	default:
		return fmt.Sprintf("https://%s:%d", oc.Registry.Host, oc.Registry.Port)
	}
}

func (itg Integrations) GetGithubConfig() *Integration {
	for _, cfg := range itg {
		if cfg.Name == "github" && cfg.Enabled {
			return cfg
		}
	}

	return &Integration{}
}

func (itg Integrations) SetGithubConfig(config *Integration) {
	for i, cfg := range itg {
		if cfg.Name == "github" {
			itg[i] = config
			break
		}
	}
}

type Environment int

const (
	Production Environment = iota
	Staging
	Local
	CI
)

func environmentFromString(env string) Environment {
	switch env {
	case Production.String():
		return Production
	case Staging.String():
		return Staging
	case Local.String():
		return Local
	case CI.String():
		return CI
	default:
		panic("deployment environment is invalid, allowed values are: PRODUCTION, STAGING, LOCAL, and CI")
	}
}

func (e Environment) String() string {
	switch e {
	case Production:
		return "PRODUCTION"
	case Staging:
		return "STAGING"
	case Local:
		return "LOCAL"
	case CI:
		return "CI"
	default:
		panic("deployment environment is invalid, allowed values are: PRODUCTION, STAGING, LOCAL, and CI")
	}
}

func (sj *Storj) S3Config() *S3CompatibleDFS {
	return &S3CompatibleDFS{
		AccessKey:       sj.AccessKey,
		SecretKey:       sj.SecretKey,
		Endpoint:        sj.Endpoint,
		BucketName:      sj.BucketName,
		DFSLinkResolver: sj.DFSLinkResolver,
		ChunkSize:       sj.ChunkSize,
		MinChunkSize:    sj.MinChunkSize,
		Enabled:         sj.Enabled,
	}
}

func (cfg *WebAppConfig) GetAllowedURLFromEchoContext(ctx echo.Context, env Environment) string {
	origin := ctx.Request().Header.Get("Origin")
	if env == Staging {
		return origin
	}

	if strings.HasSuffix(origin, "openregistry-web.pages.dev") {
		return "openregistry-web.pages.dev"
	}

	for _, url := range cfg.AllowedEndpoints {
		if url == origin {
			return url
		}
	}

	return cfg.AllowedEndpoints[0]
}

func (itg *Integration) GetAllowedURLFromEchoContext(ctx echo.Context, env Environment, allowedURLs []string) string {
	origin := ctx.Request().Header.Get("Origin")
	if env == Staging {
		return origin
	}

	if strings.HasSuffix(origin, "openregistry-web.pages.dev") {
		return "openregistry-web.pages.dev"
	}

	for _, url := range allowedURLs {
		if url == origin {
			return url
		}
	}

	return allowedURLs[0]
}

func (wan *WebAuthnConfig) GetAllowedURLFromEchoContext(ctx echo.Context, env Environment) string {
	origin := ctx.Request().Header.Get("Origin")
	if env == Staging {
		return origin
	}

	if wan.RPOrigin == origin {
		return wan.RPOrigin
	}

	for _, url := range wan.RPOrigins {
		if url == origin {
			return url
		}
	}

	if wan.RPOrigin != "" {
		return wan.RPOrigin
	}

	return wan.RPOrigins[0]
}
