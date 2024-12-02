package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_ConnectionString(t *testing.T) {
	t.Run("ConnectionString invalid scheme", func(t *testing.T) {
		cfg := Config{
			version:            "test",
			appId:              "test",
			connectionString:   "mysql://user:password@localhost:5432/db",
			dir:                "../../testing/samples/valid",
			steps:              defaultSteps,
			skipFileValidation: false,
		}

		err := cfg.validate()
		assert.ErrorIs(t, err, ErrInvalidConnectionString)
	})

	t.Run("ConnectionString invalid URL", func(t *testing.T) {
		cfg := Config{
			version:            "test",
			appId:              "test",
			connectionString:   "invalid url",
			dir:                "../../testing/samples/valid",
			steps:              defaultSteps,
			skipFileValidation: false,
		}

		err := cfg.validate()
		assert.ErrorIs(t, err, ErrInvalidConnectionString)
	})

	t.Run("ConnectionString key value", func(t *testing.T) {
		cfg := Config{
			version:            "test",
			appId:              "test",
			connectionString:   "User ID=uid;Password=pass@word;Host=url.example.com;Port=5432;Database=app;SSL Mode=allow",
			dir:                "../../testing/samples/valid",
			steps:              defaultSteps,
			skipFileValidation: false,
		}

		err := cfg.validate()
		assert.NoError(t, err)
	})

	t.Run("ConnectionString is missing", func(t *testing.T) {
		cfg := Config{
			version:            "test",
			appId:              "test",
			connectionString:   "",
			dir:                "../../testing/samples/valid",
			steps:              defaultSteps,
			skipFileValidation: false,
		}

		err := cfg.validate()
		assert.ErrorIs(t, err, ErrInvalidConnectionString)
	})
}

func TestConfig_Dir(t *testing.T) {
	t.Run("Dir is invalid", func(t *testing.T) {
		cfg := Config{
			version:            "test",
			appId:              "test",
			connectionString:   "postgres://user:password@localhost:5432/db",
			dir:                "./some/invalid/path",
			steps:              defaultSteps,
			skipFileValidation: false,
		}

		err := cfg.validate()
		assert.ErrorIs(t, err, ErrInvalidMigrationsDirectory)
	})

	t.Run("Dir is file", func(t *testing.T) {
		cfg := Config{
			version:            "test",
			appId:              "test",
			connectionString:   "postgres://user:password@localhost:5432/db",
			dir:                "../../testing/samples/exists-but-is-file",
			steps:              defaultSteps,
			skipFileValidation: false,
		}

		err := cfg.validate()
		assert.ErrorIs(t, err, ErrInvalidMigrationsDirectory)
	})

}

func TestConfig_Valid(t *testing.T) {
	cfg := Config{
		version:            "test",
		appId:              "test",
		connectionString:   "postgres://user:password@localhost:5432/db",
		dir:                "../../testing/samples/valid",
		steps:              defaultSteps,
		skipFileValidation: false,
	}

	err := cfg.validate()
	assert.NoError(t, err)
}

func TestConfig_Steps(t *testing.T) {
	cfg := Config{
		version:            "test",
		appId:              "test",
		connectionString:   "postgres://user:password@localhost:3306/db",
		dir:                "../../testing/samples/valid",
		steps:              1,
		skipFileValidation: false,
	}

	err := cfg.validate()
	assert.NoError(t, err)
}
