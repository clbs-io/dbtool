package config

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultSteps             = -1
	defaultConnectionTimeout = 45 // Seconds
)

// Config fields are not exported, making Config immutable
// Use getters to read a value from the Config struct
type Config struct {
	version string
	appId   string

	dir                    string
	connectionString       string
	connectionStringFile   string
	connectionStringFormat string
	connectionTimeout      int
	steps                  int
	skipFileValidation     bool
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

func (cfg *Config) ConnectionTimeout() int {
	return cfg.connectionTimeout
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
	flag.StringVar(&cfg.connectionStringFile, "connection-string-file", "", "Path to a file containing database URL to connect to")
	flag.StringVar(&cfg.connectionStringFormat, "connection-string-format", "default", "Connection string format. [default, ado]")
	flag.IntVar(&cfg.steps, "steps", defaultSteps, "Number of steps to apply (default: -1, apply all migrations)")
	flag.BoolVar(&cfg.skipFileValidation, "skip-file-validation", false, "Skip file validation (default: false)")
	flag.IntVar(&cfg.connectionTimeout, "connection-timeout", defaultConnectionTimeout, fmt.Sprintf("Connection timeout in seconds, must be a positive number (default: %d)", defaultConnectionTimeout))

	flag.Parse()

	if strings.ToLower(cfg.connectionStringFormat) == "ado" {
		tmp, _ := connectionStringFromADO(cfg.connectionString)
		cfg.connectionString = tmp
	}
	if cfg.connectionStringFile != "" {
		if _, err := os.Stat(cfg.connectionStringFile); err == nil {
			if data, err := os.ReadFile(cfg.connectionStringFile); err == nil {
				cfg.connectionString = strings.TrimSpace(string(data))
			}
		}
	}

	return cfg
}

func connectionStringFromADO(connectionString string) (string, bool) {
	// Split the string by semicolons
	entries := strings.Split(connectionString, ";")
	var sb strings.Builder
	for _, entry := range entries {
		// Skip empty entries (in case of trailing or multiple semicolons)
		if len(strings.TrimSpace(entry)) == 0 {
			continue
		}

		// Split key-value pairs
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			return "", false
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))

		switch key {
		case "database":
			// ADO.NET uses "Database" instead of "dbname"
			key = "dbname"
		case "user id":
			// ADO.NET uses "User ID" instead of "user"
			key = "user"
		default:
			// Remove spaces from the key as they must not be present in the PostgreSQL connection string
			key = strings.ReplaceAll(key, " ", "")
		}

		value := strings.TrimSpace(parts[1])

		// ADO.NET supports string values quoted either in single or double quotes. Go pgx does not support double-quoted values.
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			if len(value) == 2 {
				value = "''"
			} else {
				// Unescape inner double quotes, escape single quotes and add single quotes around the value
				value = fmt.Sprintf("'%s'",
					strings.ReplaceAll("'", "\\'",
						strings.ReplaceAll(value[1:len(value)-2], "\\\"", "\""),
					),
				)
			}
		}

		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(value)
		sb.WriteString(" ")
	}

	return strings.TrimSpace(sb.String()), true
}

var (
	ErrInvalidMigrationsDirectory = fmt.Errorf("invalid migrations directory path")
	ErrInvalidConnectionString    = fmt.Errorf("connection string is invalid")
	ErrInvalidSteps               = fmt.Errorf("invalid steps: must be positive integer")
	ErrInvalidAppId               = fmt.Errorf("app-id is required")
	ErrInvalidConnectionTimeout   = fmt.Errorf("connection timeout must be a positive integer")
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

	// Validate connection string by parsing it using pgxpool that has more options
	_, err = pgxpool.ParseConfig(cfg.connectionString)
	if err != nil {
		return ErrInvalidConnectionString
	}

	if cfg.steps <= 0 && cfg.steps != defaultSteps {
		return ErrInvalidSteps
	}

	if cfg.appId == "" {
		return ErrInvalidAppId
	}

	if cfg.connectionTimeout <= 0 {
		return ErrInvalidConnectionTimeout
	}

	return nil
}
