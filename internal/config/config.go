package config

import (
	"fmt"
	"time"

	"placeholder_project_tag/pkg/logging"
)

// caarlos0/env requires exported (capitalised) struct fields in order to work with these structs
type Config struct {
	Project projectConfig
	Server  serverConfig
	Domain  domainConfig
	Auth    authConfig
	Google  googleConfig
	Logging loggingConfig
	DB      dBConfig
}

type projectConfig struct {
	Env          string `env:"ENVIRONMENT" envDefault:"production"`
	Name         string `env:"PROJECT_NAME"`
	ContactEmail string `env:"CONTACT_EMAIL"`
	RepoTag      string `env:"PROJECT_REPO_TAG"`
}

type serverConfig struct {
	Host         string        `env:"API_HOST" envDefault:"localhost"`
	Port         int           `env:"API_PORT" envDefault:"8084"`
	ReadTimeout  time.Duration `env:"API_READ_TIMEOUT" envDefault:"30s"`
	WriteTimeout time.Duration `env:"API_WRITE_TIMEOUT" envDefault:"30s"`
	IdleTimeout  time.Duration `env:"API_IDLE_TIMEOUT" envDefault:"1m"`
	Limiter      limiterConfig
	TLS          tlsConfig
}

// rps (requests per second) must be float, burst must be int for limiter. enabled allows turning off the rate limiter for, for example load testing.
type limiterConfig struct {
	Enabled bool    `env:"API_LIMITER_ENABLED" envDefault:"false"`
	RPS     float64 `env:"API_LIMITER_RPS" envDefault:"2"`
	Burst   int     `env:"API_LIMITER_BURST" envDefault:"4"`
}

type domainConfig struct {
	DomainMain string `env:"DOMAIN_MAIN" envDefault:"localhost"`
}

type tlsConfig struct {
	HTTPSOn  bool   `env:"HTTPS_ON" envDefault:"false"`
	CertPath string `env:"CERT_PATH" envDefault:"./tls/cert.pem"`
	KeyPath  string `env:"KEY_PATH" envDefault:"./tls/key.pem"`
}

type authConfig struct {
	JWTSecret     string        `env:"JWT_SECRET" required:"true"`
	JWTExpiration time.Duration `env:"JWT_EXPIRATION" envDefault:"24h"`
	JWTRefresh    time.Duration `env:"JWT_REFRESH" envDefault:"6h"`
	BCryptCost    int           `env:"BCRYPT_COST" envDefault:"12"`
}

type googleConfig struct {
	GoogleClientID     string `env:"GOOGLE_CLIENT_ID" required:"true"`
	GoogleClientSecret string `env:"GOOGLE_CLIENT_SECRET" required:"true"`
	GoogleRedirectURL  string `env:"GOOGLE_REDIRECT_URL" envDefault:"https://localhost/oauth/google/callback"`
}

type loggingConfig struct {
	Level   logging.Level `env:"LOG_LEVEL" envDefault:"1"`
	LogToDB bool          `env:"LOG_TO_DB" envDefault:"false"`
	// Format string `env:"LOG_FORMAT" envDefault:"json"`
}

type dBConfig struct {
	BackUpEnabled bool   `env:"BACKUP_ENABLED" envDefault:"false"`
	AppDBPath     string `env:"APP_DB_PATH"`
	MonitorDBPath string `env:"MONITOR_DB_PATH"`
}

// Validate performs validation on the configuration
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535, got %d", c.Server.Port)
	}
	if c.Server.Limiter.RPS < 1 {
		return fmt.Errorf("server limiter rps must be at least 1, got %f", c.Server.Limiter.RPS)
	}
	if c.Server.Limiter.Burst < 1 {
		return fmt.Errorf("server limiter burst must be at least 1, got %d", c.Server.Limiter.Burst)
	}

	// Validate auth config
	if c.Auth.BCryptCost < 4 || c.Auth.BCryptCost > 31 {
		return fmt.Errorf("bcrypt cost must be between 4 and 31, got %d", c.Auth.BCryptCost)
	}

	return nil
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.Project.Env == "development"
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Project.Env == "production"
}

// IsStaging returns true if running in staging environment
func (c *Config) IsStaging() bool {
	return c.Project.Env == "staging"
}

// Print prints the configuration (with sensitive data masked)
func (c *Config) Print() {
	fmt.Printf("Server: %s:%d (env: %s)\n", c.Server.Host, c.Server.Port, c.Project.Env)
	fmt.Printf("Logging: %d level (log to database: %t)\n", c.Logging.Level, c.Logging.LogToDB)
}
