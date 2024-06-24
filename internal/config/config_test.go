package config

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConfig_DatabaseURL(t *testing.T) {
	t.Run("DatabaseURL invalid scheme", func(t *testing.T) {
		cfg := Config{
			databaseURL:        "mysql://user:password@localhost:5432/db",
			dir:                "../../testing/samples/valid",
			steps:              defaultSteps,
			skipFileValidation: false,
		}

		err := cfg.validate()
		assert.ErrorIs(t, err, ErrInvalidDatabaseURL)
	})

	t.Run("DatabaseURL invalid URL", func(t *testing.T) {
		cfg := Config{
			databaseURL:        "invalid url",
			dir:                "../../testing/samples/valid",
			steps:              defaultSteps,
			skipFileValidation: false,
		}

		err := cfg.validate()
		assert.ErrorIs(t, err, ErrInvalidDatabaseURL)
	})

	t.Run("DatabaseURL is missing", func(t *testing.T) {
		cfg := Config{
			databaseURL:        "",
			dir:                "../../testing/samples/valid",
			steps:              defaultSteps,
			skipFileValidation: false,
		}

		err := cfg.validate()
		assert.ErrorIs(t, err, ErrInvalidDatabaseURL)
	})
}

func TestConfig_Dir(t *testing.T) {
	t.Run("Dir is invalid", func(t *testing.T) {
		cfg := Config{
			databaseURL:        "postgres://user:password@localhost:5432/db",
			dir:                "./some/invalid/path",
			steps:              defaultSteps,
			skipFileValidation: false,
		}

		err := cfg.validate()
		assert.ErrorIs(t, err, ErrInvalidMigrationsDirectory)
	})

	t.Run("Dir is file", func(t *testing.T) {
		cfg := Config{
			databaseURL:        "postgres://user:password@localhost:5432/db",
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
		databaseURL:        "postgres://user:password@localhost:5432/db",
		dir:                "../../testing/samples/valid",
		steps:              defaultSteps,
		skipFileValidation: false,
	}

	err := cfg.validate()
	assert.NoError(t, err)
}

func TestConfig_Steps(t *testing.T) {
	cfg := Config{
		databaseURL:        "postgres://user:password@localhost:3306/db",
		dir:                "../../testing/samples/valid",
		steps:              1,
		skipFileValidation: false,
	}

	err := cfg.validate()
	assert.NoError(t, err)
}
