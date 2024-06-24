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

func LoadConfig() (*Config, error) {
	cfg := load()
	err := cfg.validate()
	return cfg, err
}

func load() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.dir, "migrations-dir", "", "Root directory where to look for SQL files")
	flag.StringVar(&cfg.databaseURL, "database-url", "", "Database URL to connect to")
	flag.IntVar(&cfg.steps, "steps", defaultSteps, "Number of steps to apply")
	flag.BoolVar(&cfg.skipFileValidation, "skip-file-validation", false, "Skip file validation")

	flag.Parse()

	return cfg
}

func (cfg *Config) validate() error {
	if cfg.dir == "" {
		return fmt.Errorf("migrations-dir is required")
	}

	fileInfo, err := os.Stat(cfg.dir)
	if err != nil {
		return fmt.Errorf("dir %s does not exist", cfg.dir)
	}

	if !fileInfo.IsDir() {
		return fmt.Errorf("dir %s is not a directory", cfg.dir)
	}

	if cfg.databaseURL == "" {
		return fmt.Errorf("database-url is required")
	}

	// use url.ParseRequestURI() to validate the URL, not url.Parse(), since almost anything is valid for url.Parse()
	parsedURL, err := url.ParseRequestURI(cfg.databaseURL)
	if err != nil {
		return fmt.Errorf("database-url is not a valid URL")
	}

	if parsedURL.Scheme != "postgres" {
		return fmt.Errorf("database-url scheme must be 'postgres'")
	}

	return nil
}
