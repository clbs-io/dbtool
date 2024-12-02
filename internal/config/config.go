package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
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
	connectionString   string
	steps              int
	skipFileValidation bool
}

func (cfg *Config) Dir() string {
	return cfg.dir
}

func (cfg *Config) ConnectionString() string {
	return cfg.connectionString
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
	cfg.version = version
	err := cfg.validate()
	return cfg, err
}

func load() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.appId, "app-id", "", "Application ID")
	flag.StringVar(&cfg.dir, "migrations-dir", "", "Root directory where to look for SQL files")
	flag.StringVar(&cfg.connectionString, "connection-string", "", "Database URL to connect to")
	flag.IntVar(&cfg.steps, "steps", defaultSteps, "Number of steps to apply")
	flag.BoolVar(&cfg.skipFileValidation, "skip-file-validation", false, "Skip file validation")

	flag.Parse()

	return cfg
}

var (
	ErrInvalidMigrationsDirectory = fmt.Errorf("invalid migrations directory path")
	ErrInvalidConnectionString    = fmt.Errorf("connection string is invalid")
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

	if cfg.connectionString == "" {
		return ErrInvalidConnectionString
	}

	_, err = pgx.ParseConfig(cfg.connectionString)
	if err != nil {
		return ErrInvalidConnectionString
	}

	if cfg.steps <= 0 && cfg.steps != defaultSteps {
		return ErrInvalidSteps
	}

	if cfg.appId == "" {
		return ErrInvalidAppId
	}

	return nil
}
