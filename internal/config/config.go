package config

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"gopkg.in/yaml.v2"
)

type BoardConfig struct {
	Slug        string `yaml:"slug"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type Config struct {
	Boards []BoardConfig `yaml:"boards"`
}

func LoadConfig(filePath string) (*Config, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	// Validate config
	for _, board := range config.Boards {
		if board.Slug == "" || board.Name == "" {
			return nil, fmt.Errorf("invalid board configuration: slug and name are required")
		}
	}

	return &config, nil
}

func InitializeBoards(config *Config, db *sql.DB) error {
	stmt, err := db.Prepare(`
		INSERT INTO boards (slug, name, description)
		VALUES ($1, $2, $3)
		ON CONFLICT (slug) DO UPDATE
		SET name = EXCLUDED.name,
		    description = EXCLUDED.description
		RETURNING id
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, board := range config.Boards {
		var id int
		err := stmt.QueryRow(board.Slug, board.Name, board.Description).Scan(&id)
		if err != nil {
			return fmt.Errorf("failed to upsert board %s: %w", board.Slug, err)
		}
		log.Printf("Initialized board: %s (ID: %d)", board.Name, id)
	}

	return nil
}

type DBConfig struct {
	Host            string `yaml:"host"`
	Port            string `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	Name            string `yaml:"name"`
	SSLMode         string `yaml:"sslmode"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"`
}

func LoadDBConfig(filePath string) (*DBConfig, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading DB config file: %w", err)
	}

	var config struct {
		Database DBConfig `yaml:"database"`
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing DB config file: %w", err)
	}

	// Validate required fields
	if config.Database.Host == "" || config.Database.User == "" || config.Database.Name == "" {
		return nil, fmt.Errorf("database configuration requires host, user, and name")
	}

	return &config.Database, nil
}

type SiteConfig struct {
	Title       string `yaml:"title"`
	Tagline     string `yaml:"tagline"`
	Description string `yaml:"description"`
	Greeting    string `yaml:"greeting"`
	FooterText  string `yaml:"footer_text"`
}

func LoadSiteConfig(filePath string) (*SiteConfig, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading site config: %w", err)
	}

	var config struct {
		Site SiteConfig `yaml:"site"`
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing site config: %w", err)
	}

	// Validate required fields
	if config.Site.Title == "" || config.Site.Description == "" {
		return nil, fmt.Errorf("site configuration requires title and description")
	}

	return &config.Site, nil
}

type SessionSecurity struct {
	SecretKey  string `yaml:"secret_key"`
	CookieName string `yaml:"cookie_name"`
	MaxAge     int    `yaml:"max_age"`
	Secure     bool   `yaml:"secure"`
	HTTPOnly   bool   `yaml:"http_only"`
	SameSite   string `yaml:"same_site"`

	CSRF struct {
		Secret    string `yaml:"secret"`
		FieldName string `yaml:"field_name"`
		Header    string `yaml:"header"`
		Expires   int    `yaml:"expires"`
		ErrorMsg  string `yaml:"error_message"`
	} `yaml:"csrf"`
}

func LoadSessionConfig(filePath string) (*SessionSecurity, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading session config: %w", err)
	}

	var config struct {
		Session SessionSecurity `yaml:"session"`
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing session config: %w", err)
	}

	if config.Session.SecretKey == "" {
		return nil, fmt.Errorf("session secret_key is required")
	}

	if config.Session.CSRF.Secret == "" {
		return nil, fmt.Errorf("csrf.secret is required")
	}

	if config.Session.CSRF.Secret == "your-csrf-secret" {
		return nil, fmt.Errorf("you must change the default csrf secret")
	}

	return &config.Session, nil
}

func (s *SessionSecurity) SameSiteMode() http.SameSite {
	switch strings.ToLower(s.SameSite) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

type RateLimitConfig struct {
	MaxRequests        int    `yaml:"max_requests"`
	WindowSeconds      int    `yaml:"window_seconds"`
	CleanupSeconds     int    `yaml:"cleanup_seconds"`
	EnableJSONResponse bool   `yaml:"enable_json_response"`
	ErrorMessage       string `yaml:"error_message"`
	ErrorResponse      struct {
		Code    int               `yaml:"code"`
		Headers map[string]string `yaml:"headers"`
		Body    string            `yaml:"body"`
	} `yaml:"error_response"`
}

func LoadRateLimitConfig(filePath string) (*RateLimitConfig, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading rate limit config: %w", err)
	}

	var config struct {
		RateLimits RateLimitConfig `yaml:"rate_limits"`
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing rate limit config: %w", err)
	}

	if config.RateLimits.MaxRequests < 1 {
		return nil, fmt.Errorf("max_requests must be at least 1")
	}

	return &config.RateLimits, nil
}
