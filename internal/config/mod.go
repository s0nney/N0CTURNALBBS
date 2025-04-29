package config

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"
)

type ModeratorConfig struct {
	Moderators []struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"` // Stored as bcrypt hash in the config
		IsActive bool   `yaml:"is_active"`
	} `yaml:"moderators"`

	Session struct {
		CookieName string `yaml:"cookie_name"`
		MaxAge     int    `yaml:"max_age"`
		Secure     bool   `yaml:"secure"`
		HTTPOnly   bool   `yaml:"http_only"`
		SameSite   string `yaml:"same_site"`
	} `yaml:"session"`

	Security struct {
		BcryptCost        int `yaml:"bcrypt_cost"`
		MinPasswordLength int `yaml:"min_password_length"`
		MaxLoginAttempts  int `yaml:"max_login_attempts"`
		LockoutDuration   int `yaml:"lockout_duration"`
	} `yaml:"security"`
}

func LoadModConfig(filePath string) (*ModeratorConfig, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading moderator config file: %w", err)
	}

	var config ModeratorConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing moderator config file: %w", err)
	}

	if len(config.Moderators) == 0 {
		return nil, fmt.Errorf("no moderators defined in config")
	}

	return &config, nil
}

func InitializeModerators(config *ModeratorConfig, db *sql.DB) error {
	for _, mod := range config.Moderators {
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM moderators WHERE username = $1)", mod.Username).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check if moderator exists: %w", err)
		}

		if exists {
			_, err = db.Exec("UPDATE moderators SET is_active = $1 WHERE username = $2",
				mod.IsActive, mod.Username)
			if err != nil {
				return fmt.Errorf("failed to update moderator %s: %w", mod.Username, err)
			}
			log.Printf("Updated moderator: %s", mod.Username)
		} else {
			_, err = db.Exec(`
				INSERT INTO moderators (username, password_hash, is_active, created_at)
				VALUES ($1, $2, $3, $4)`,
				mod.Username, mod.Password, mod.IsActive, time.Now(),
			)
			if err != nil {
				return fmt.Errorf("failed to create moderator %s: %w", mod.Username, err)
			}
			log.Printf("Created moderator: %s", mod.Username)
		}
	}

	return nil
}

func HashPassword(password string, cost int) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(bytes), err
}

func GeneratePasswordHash(password string) (string, error) {
	return HashPassword(password, 10)
}
