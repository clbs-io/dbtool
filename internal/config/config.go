package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"
)

const (
	defaultSteps = -1
)

// Config fields are not exported, making Config immutable
// Use getters to read a value from the Config struct
type Config struct {
	version string
	appId   string

	dir                string
	databaseURL        string
	steps              int
	skipFileValidation bool
}

func (cfg *Config) Dir() string {
	return cfg.dir
}

func (cfg *Config) DatabaseURL() string {
	return cfg.databaseURL
}

func (cfg *Config) Steps() int {
	return cfg.steps
}

func (cfg *Config) SkipFileValidation() bool {
	return cfg.skipFileValidation
}

func (cfg *Config) Version() string {
	return cfg.version
}

func (cfg *Config) AppId() string {
	return cfg.appId
}

func LoadConfig(version string) (*Config, error) {
	cfg := load()
	err := cfg.validate()
	return cfg, err
}

func load() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.appId, "app-id", "", "Application ID")
	flag.StringVar(&cfg.dir, "migrations-dir", "", "Root directory where to look for SQL files")
	flag.StringVar(&cfg.databaseURL, "database-url", "", "Database URL to connect to")
	flag.IntVar(&cfg.steps, "steps", defaultSteps, "Number of steps to apply")
	flag.BoolVar(&cfg.skipFileValidation, "skip-file-validation", false, "Skip file validation")

	flag.Parse()

	return cfg
}

var (
	ErrInvalidMigrationsDirectory = fmt.Errorf("invalid migrations directory path")
	ErrInvalidDatabaseURL         = fmt.Errorf("database url is invalid")
	ErrInvalidSteps               = fmt.Errorf("invalid steps: must be positive integer")
	ErrInvalidAppId               = fmt.Errorf("app-id is required")
)

func (cfg *Config) validate() error {
	if cfg.dir == "" {
		return ErrInvalidMigrationsDirectory
	}

	fileInfo, err := os.Stat(cfg.dir)
	if err != nil {
		return ErrInvalidMigrationsDirectory
	}

	if !fileInfo.IsDir() {
		return ErrInvalidMigrationsDirectory
	}

	// use url.ParseRequestURI() to validate the URL, not url.Parse(), since almost anything is valid for url.Parse()
	parsedURL, err := url.ParseRequestURI(cfg.databaseURL)
	if err != nil {
		return ErrInvalidDatabaseURL
	}

	if parsedURL.Scheme != "postgres" {
		return ErrInvalidDatabaseURL
	}

	if cfg.steps <= 0 && cfg.steps != defaultSteps {
		return ErrInvalidSteps
	}

	if cfg.appId == "" {
		return ErrInvalidAppId
	}

	return nil
}
