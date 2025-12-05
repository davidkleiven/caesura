package pkg

import (
	"embed"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/davidkleiven/caesura/utils"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gopkg.in/yaml.v2"
)

const GoogleCloud = "google-cloud"

//go:embed profiles/*
var configProfiles embed.FS

type LocalFSStoreConfig struct {
	Directory string `yaml:"directory"`
	Database  string `yaml:"database"`
}

type Smtp struct {
	Auth   smtp.Auth
	Host   string   `yaml:"host"`
	Port   string   `yaml:"port"`
	SendFn SendFunc `yaml:"-"`
}

func NewBrevo(password string) *Smtp {
	host := "smtp-relay.brevo.com"
	return &Smtp{
		Host:   host,
		Port:   "587",
		Auth:   smtp.PlainAuth("", "9ac97b001@smtp-brevo.com", password, host),
		SendFn: smtp.SendMail,
	}
}

type GoogleClientContainer struct {
	FirestoreClient  *firestore.Client
	CloudStoreClient *storage.Client
}

type PriceIds struct {
	Free    string
	Monthly string
	Annual  string
}

func (p *PriceIds) NumScores(priceId string) int {
	switch priceId {
	case p.Monthly, p.Annual:
		return 500
	default:
		return 10
	}
}

func (p *PriceIds) PriceIdFromSubscriptionPlan(plan string) string {
	switch plan {
	case "monthly":
		return p.Monthly
	case "annual":
		return p.Annual
	default:
		return p.Free
	}
}

func NewTestPriceIds() *PriceIds {
	return &PriceIds{
		Free:    "price_1RvOBAF9NBcrR1kwWkhZVwwX",
		Monthly: "price_1RvOAWF9NBcrR1kwDySNEUFE",
		Annual:  "price_1RvObkF9NBcrR1kwBHiYsagO",
	}
}

func NewProdPriceIds() *PriceIds {
	return &PriceIds{
		Free:    "price_1Sb2jaFUof2YkdVthkjEGhGy",
		Monthly: "price_1Sb2kCFUof2YkdVtpImXByRc",
		Annual:  "price_1Sb2kdFUof2YkdVt7uSVAHa2",
	}
}

type Config struct {
	StoreType                string                `yaml:"store_type" env:"CAESURA_STORE_TYPE"`
	LocalFS                  LocalFSStoreConfig    `yaml:"local_fs"`
	Timeout                  time.Duration         `yaml:"timeout" env:"CAESURA_TIMEOUT"`
	Port                     int                   `yaml:"port" env:"CAESURA_PORT"`
	SecretsPath              string                `yaml:"secrets_path" env:"CAESURA_SECRETS_PATH"`
	MaxRequestSizeMb         uint                  `yaml:"max_request_size_mb" env:"CAESURA_MAX_REQUEST_SIZE_MB"`
	GoogleAuthClientId       string                `yaml:"google_auth_client_id" env:"CAESURA_GOOGLE_AUTH_CLIENT_ID"`
	GoogleAuthClientSecretId string                `yaml:"google_auth_client_secret_id" env:"CAESURA_GOOGLE_AUTH_CLIENT_SECRET_ID"`
	GoogleAuthRedirectURL    string                `yaml:"google_auth_rederict_url" env:"CAESURA_GOOGLE_AUTH_REDIRECT_URL"`
	CookieSecretSignKey      string                `yaml:"cookie_secret_sign_key" env:"CAESURA_COOKIE_SECRET_SIGN_KEY"`
	BaseURL                  string                `yaml:"base_url" env:"CAESURA_BASE_URL"`
	SessionMaxAge            int                   `yaml:"session_max_age" env:"CAESURA_SESSION_MAX_AGE"`
	SmtpConfig               Smtp                  `yaml:"smtp"`
	EmailSender              string                `yaml:"email_sender" env:"CAESURA_EMAIL_SENDER"`
	StripeSecretKey          string                `yaml:"stripe_secret_key" env:"CAESURA_STRIPE_SECRET_KEY"`
	StripeWebhookSignSecret  string                `yaml:"stripe_webhook_sign_secret" env:"CAESURA_STRIPE_WEBHOOK_SIGN_SECRET"`
	StripeIdProvider         string                `yaml:"stripe_id_provider" env:"CAESURA_STRIPE_ID_PROVIDER"`
	RequireSubscription      bool                  `yaml:"require_subscription" env:"CAUSURA_REQUIRE_SUBSCRIPTION"`
	BrevoApiKey              string                `yaml:"brevo_api_key" env:"CAESURA_BREVO_API_KEY"`
	EmailDeliveryService     string                `yaml:"email_delivery_service" env:"CAESURA_EMAIL_DELIVERY_SERVICE"`
	GoogleCfg                GoogleConfig          `yaml:"google_config"`
	PortalSessionProvider    string                `yaml:"portal_session_provider"`
	Transport                http.RoundTripper     `yaml:"-"`
	GoogleClients            GoogleClientContainer `yaml:"-"`
}

func (c *Config) Validate() error {
	switch c.StoreType {
	case "in-memory", GoogleCloud:
		// No additional validation needed for in-memory store
	case "small-demo":
		// No additional validation
	case "large-demo":
		// No additional validation
	case "local-fs":
		if c.LocalFS.Directory == "" {
			return fmt.Errorf("local_fs.directory must be specified for local-fs store")
		}
	default:
		return fmt.Errorf("unknown store_type: %s", c.StoreType)
	}
	return nil
}

func (c *Config) OAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.GoogleAuthClientId,
		ClientSecret: c.GoogleAuthClientSecretId,
		RedirectURL:  c.GoogleAuthRedirectURL,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
}

func (c *Config) SessionOpts() *sessions.Options {
	return &sessions.Options{
		Path:   "/",
		MaxAge: c.SessionMaxAge,
	}
}

func (c *Config) GetStripeIdProvider() StripeCustomerIdProvider {
	switch c.StripeIdProvider {
	case "stripe":
		slog.Info("Using 'stripe' as provider of customer ids")
		return &PaymentSystemCusteromIdProvider{ApiKey: c.StripeSecretKey}
	default:
		slog.Info("Using local stripe provider. Should not be used in production!")
		return &LocalStripeCustomerIdProvider{}
	}
}

func (c *Config) GetPortalSessionProvider() BillingPortalSessionProvider {
	switch c.PortalSessionProvider {
	case "fixed":
		slog.Info("Using local portal session provider")
		return &FixedPortalSessionProvider{Url: "http://customer-portal.no"}
	default:
		return &StripeBillingSessionProvider{ApiKey: c.StripeSecretKey}
	}
}

func (c *Config) GetPriceIds() *PriceIds {
	switch c.GoogleCfg.Environment {
	case "prod":
		return NewProdPriceIds()
	default:
		return NewTestPriceIds()
	}
}

func NewDefaultConfig() *Config {
	return &Config{
		StoreType:             "in-memory",
		Timeout:               10 * time.Second,
		Port:                  8080,
		MaxRequestSizeMb:      100,
		GoogleAuthClientId:    "602223566336-77ugev7r0br5k1j8rc8i407kb0et34al.apps.googleusercontent.com",
		GoogleAuthRedirectURL: "http://localhost:8080/auth/callback",
		BaseURL:               "http://localhost:8080",
		SessionMaxAge:         3600,
		SmtpConfig: Smtp{
			SendFn: smtp.SendMail,
		},
	}
}

func OverrideFromFile(filePath string, config *Config) (*Config, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return config, fmt.Errorf("error opening config file %s: %w", filePath, err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return config, fmt.Errorf("error reading config file %s: %w", filePath, err)
	}

	if err := yaml.Unmarshal(content, config); err != nil {
		return config, fmt.Errorf("error parsing config file %s: %w", filePath, err)
	}
	return config, nil
}

type EnvGetter func(key string) (string, bool)

// OverrideFromEnv asks all getters in the passed
func OverrideFromEnv[T any](config *T, getter EnvGetter) *T {
	t := reflect.TypeOf(config).Elem()
	v := reflect.ValueOf(config).Elem()

	numLoaded := 0
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)
		envTag := field.Tag.Get("env")
		if envTag == "" || !fieldValue.CanSet() {
			continue
		}

		value, ok := getter(envTag)
		if !ok {
			continue
		}

		switch fieldValue.Kind() {
		case reflect.String:
			fieldValue.SetString(value)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			intVal := utils.Must(strconv.ParseInt(value, 10, 64))
			fieldValue.SetInt(intVal)
		}
		numLoaded++
	}

	slog.Info("Loaded variables from environment", "num", numLoaded)
	return config
}

func FileEnvGetter(path string) EnvGetter {
	return func(key string) (string, bool) {
		f, err := os.Open(filepath.Join(path, key))
		if err != nil {
			return "", false
		}
		defer f.Close()
		value, err := io.ReadAll(f)
		if err != nil {
			return "", false
		}
		return string(value), true
	}
}

func LoadConfig(configFile string) (*Config, error) {
	config := NewDefaultConfig()
	if configFile != "" {
		if _, err := OverrideFromFile(configFile, config); err != nil {
			return config, err
		}
	}
	OverrideFromEnv(config, os.LookupEnv)
	OverrideFromEnv(config, FileEnvGetter(config.SecretsPath))
	return OverrideEmailDeliveryService(config)
}

func OverrideEmailDeliveryService(config *Config) (*Config, error) {
	switch config.EmailDeliveryService {
	case "brevo":
		if config.BrevoApiKey == "" {
			return config, fmt.Errorf("Email delivery service was 'brevo', but no api key was provided")
		}
		config.SmtpConfig = *NewBrevo(config.BrevoApiKey)
		config.EmailSender = "noreply@caesura.no"
	}
	return config, nil
}

func GetStore(config *Config) Store {
	msg := "Getting store for config"
	key := "store"
	switch config.StoreType {
	case "small-demo":
		slog.Info(msg, key, "small-demo")
		return NewDemoStore()
	case "large-demo":
		slog.Info(msg, key, "large-demo")
		return NewLargeDemoStore()
	case GoogleCloud:
		slog.Info(msg, key, "google-cloud")
		return &GoogleStore{
			FsClient: &GoogleFirestoreClient{
				client:      config.GoogleClients.FirestoreClient,
				environment: config.GoogleCfg.Environment},
			BucketClient: &GCSBucketClient{client: config.GoogleClients.CloudStoreClient},
		}
	default:
		slog.Info(msg, key, "empty-store")
		return NewMultiOrgInMemoryStore()
	}
}

func LoadProfile(name string) (*Config, error) {
	config := NewDefaultConfig()
	data, err := configProfiles.ReadFile(fmt.Sprintf("profiles/%s", name))
	if err != nil {
		return config, fmt.Errorf("Could not open file %s: %w", name, err)
	}

	cleartext, err := decrypt.Data(data, "yaml")
	if err != nil {
		return config, fmt.Errorf("Could not decrypt config file %s: %w", name, err)
	}
	err = yaml.Unmarshal(cleartext, config)
	return config, err
}
