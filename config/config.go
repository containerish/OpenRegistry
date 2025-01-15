package config

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/go-multierror"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type (
	OpenRegistryConfig struct {
		OAuth          OAuth          `yaml:"oauth" mapstructure:"oauth" validate:"-"`
		Email          Email          `yaml:"email" mapstructure:"email" validate:"-"`
		WebAppConfig   WebAppConfig   `yaml:"web_app" mapstructure:"web_app"`
		Integrations   Integrations   `yaml:"integrations" mapstructure:"integrations"`
		Registry       Registry       `yaml:"registry" mapstructure:"registry" validate:"required"`
		StoreConfig    Store          `yaml:"database" mapstructure:"database" validate:"required"`
		Telemetry      Telemetry      `yaml:"telemetry" mapstructure:"telemetry"`
		WebAuthnConfig WebAuthnConfig `yaml:"web_authn_config" mapstructure:"web_authn_config"`
		DFS            DFS            `yaml:"dfs" mapstructure:"dfs"`
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
		Storj    Storj           `yaml:"storj" mapstructure:"storj"`
		Filebase S3CompatibleDFS `yaml:"filebase" mapstructure:"filebase"`
		Ipfs     IpfsDFS         `yaml:"ipfs" mapstructure:"ipfs"`
		Mock     S3CompatibleDFS `yaml:"mock" mapstructure:"mock"`
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

		// this field is only used by the mock storage driver
		Type MockStorageBackend `yaml:"type" mapstructure:"type"`
	}

	MockStorageBackend int

	// just so that we can retrieve values easily
	Integrations map[string]any

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

	Store struct {
		Kind               StoreKind `yaml:"kind" mapstructure:"kind" validate:"required"`
		User               string    `yaml:"username" mapstructure:"username" validate:"required"`
		Host               string    `yaml:"host" mapstructure:"host" validate:"required"`
		Password           string    `yaml:"password" mapstructure:"password" validate:"required"`
		Database           string    `yaml:"name" mapstructure:"name" validate:"required"`
		MaxOpenConnections int       `yaml:"max_open_connections" mapstructure:"max_open_connections" validate:"-"`
		Port               int       `yaml:"port" mapstructure:"port" validate:"required"`
	}

	StoreKind string

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

	GithubIntegration struct {
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

	ClairIntegration struct {
		ClairEndpoint string `yaml:"clair_endpoint" mapstructure:"clair_endpoint"`
		AuthToken     string `yaml:"auth_token" mapstructure:"auth_token"`
		Host          string `yaml:"host" mapstructure:"host"`
		Port          int    `yaml:"port" mapstructure:"port"`
		Enabled       bool   `yaml:"enabled" mapstructure:"enabled"`
	}

	Auth struct {
		JWTSigningPrivateKey *rsa.PrivateKey `yaml:"priv_key" mapstructure:"priv_key"`
		JWTSigningPubKey     *rsa.PublicKey  `yaml:"pub_key" mapstructure:"pub_key"`
		Enabled              bool            `yaml:"enabled" mapstructure:"enabled"`
	}

	Otel struct {
		Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	}

	AxiomConfig struct {
		Dataset        string `yaml:"dataset" mapstructure:"dataset"`
		APIKey         string `yaml:"api_key" mapstructure:"api_key"`
		OrganizationID string `yaml:"organization_id" mapstructure:"organization_id"`
		Enabled        bool   `yaml:"enabled" mapstructure:"enabled"`
	}

	FluentBitConfig struct {
		Endpoint   string `yaml:"endpoint" mapstructure:"endpoint"`
		AuthMethod string `yaml:"auth_method" mapstructure:"auth_method"`
		Username   string `yaml:"username" mapstructure:"username"`
		Password   string `yaml:"password" mapstructure:"password"`
		Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`
	}

	Logging struct {
		Axiom            AxiomConfig     `yaml:"axiom" mapstructure:"axiom"`
		Level            string          `yaml:"level" mapstructure:"level"`
		FluentBit        FluentBitConfig `yaml:"fluent_bit" mapstructure:"fluent_bit"`
		Pretty           bool            `yaml:"pretty" mapstructure:"pretty"`
		RemoteForwarding bool            `yaml:"remote_forwarding" mapstructure:"remote_forwarding"`
		Enabled          bool            `yaml:"enabled" mapstructure:"enabled"`
	}

	Telemetry struct {
		Honeycomb Honeycomb `yaml:"honeycomb" mapstructure:"honeycomb"`
		Logging   Logging   `yaml:"logging" mapstructure:"logging"`
		Otel      Otel      `yaml:"otel" mapstructure:"otel"`
		Enabled   bool      `yaml:"enabled" mapstructure:"enabled"`
	}

	Honeycomb struct {
		ServiceName string `yaml:"service_name" mapstructure:"service_name"`
		ApiKey      string `yaml:"api_key" mapstructure:"api_key"`
		Enabled     bool   `yaml:"enabled" mapstructure:"enabled"`
	}

	IpfsDFS struct {
		RPCMultiAddr    string `yaml:"rpc_multi_addr" mapstructure:"rpc_multi_addr"`
		GatewayEndpoint string `yaml:"gateway_endpoint" mapstructure:"gateway_endpoint"`
		Type            string `yaml:"type" mapstructure:"type"`
		Enabled         bool   `yaml:"enabled" mapstructure:"enabled"`
		Local           bool   `yaml:"local" mapstructure:"local"`
		Pinning         bool   `yaml:"pinning" mapstructure:"pinning"`
	}
)

const (
	StoreKindPostgres StoreKind = "postgres"
	StoreKindSQLite   StoreKind = "sqlite"

	MaxS3UploadParts int32 = 1000

	MockStorageBackendMemMapped MockStorageBackend = iota + 1
	MockStorageBackendFileBased
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
			translatedErr = multierror.Append(translatedErr, errors.New(e.Translate(trans)))
		}

		return translatedErr
	}

	return nil
}

func (sc *Store) Endpoint() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
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
	default:
		return fmt.Sprintf("https://%s:%d", oc.Registry.Host, oc.Registry.Port)
	}
}

func (itg Integrations) GetClairConfig() *ClairIntegration {
	cfg := ClairIntegration{
		Enabled: false,
	}
	clairCfg, ok := itg["clair"]
	if !ok {
		return &cfg
	}

	bz, err := yaml.Marshal(clairCfg)
	if err != nil {
		color.Red("error reading clair integration config as json: %s", err)
		return nil
	}

	if err = yaml.Unmarshal(bz, &cfg); err != nil {
		color.Red("error parsing Clair integration config: %s", err)
		return nil
	}

	if cfg.Host == "" {
		cfg.Host = "localhost"
	}

	if cfg.Port == 0 {
		cfg.Port = 5004
	}

	return &cfg
}

func (itg Integrations) GetGithubConfig() *GithubIntegration {
	cfg := GithubIntegration{
		Enabled: false,
	}
	ghCfg, ok := itg["github"]
	if !ok {
		return &cfg
	}

	bz, err := yaml.Marshal(ghCfg)
	if err != nil {
		color.Red("error reading github integration config as json: %s", err)
		return nil
	}

	if err = yaml.Unmarshal(bz, &cfg); err != nil {
		color.Red("error parsing GitHub integration config: %s", err)
		return nil
	}

	if cfg.Host == "" {
		cfg.Host = "localhost"
	}

	if cfg.Port == 0 {
		cfg.Port = 5001
	}

	return &cfg
}

func (itg Integrations) SetGithubConfig(config map[string]any) {
	itg["github"] = config
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

func (itg *GithubIntegration) GetAllowedURLFromEchoContext(
	ctx echo.Context,
	env Environment,
	allowedURLs []string,
) string {
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

func (a *Auth) keyIDEncode(b []byte) string {
	s := strings.TrimRight(base32.StdEncoding.EncodeToString(b), "=")
	var buf bytes.Buffer
	var i int
	for i = 0; i < len(s)/4-1; i++ {
		start := i * 4
		end := start + 4
		buf.WriteString(s[start:end] + ":")
	}
	buf.WriteString(s[i*4:])
	return buf.String()
}

func (a *Auth) SignWithPubKey(claims jwt.Claims) (string, error) {
	pubKeyDerBz, err := x509.MarshalPKIXPublicKey(a.JWTSigningPubKey)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write(pubKeyDerBz)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = a.keyIDEncode(hasher.Sum(nil)[:30])
	signed, err := token.SignedString(a.JWTSigningPrivateKey)
	if err != nil {
		return "", fmt.Errorf("error signing secret %w", err)
	}

	return signed, nil
}
