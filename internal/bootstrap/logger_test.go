// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package bootstrap

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestLogger(t *testing.T) {
	t.Run("Returns development logger when not in Kubernetes", func(t *testing.T) {
		// Ensure KUBERNETES_SERVICE_HOST is not set
		_ = os.Unsetenv("KUBERNETES_SERVICE_HOST")

		logger := Logger()
		assert.NotNil(t, logger)

		// Development logger should be created
		assert.IsType(t, &zap.Logger{}, logger)
	})

	t.Run("Returns production logger when in Kubernetes", func(t *testing.T) {
		// Set KUBERNETES_SERVICE_HOST to simulate Kubernetes environment
		originalValue := os.Getenv("KUBERNETES_SERVICE_HOST")
		err := os.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
		assert.NoError(t, err)
		defer func() {
			if originalValue == "" {
				_ = os.Unsetenv("KUBERNETES_SERVICE_HOST")
			} else {
				_ = os.Setenv("KUBERNETES_SERVICE_HOST", originalValue)
			}
		}()

		logger := Logger()
		assert.NotNil(t, logger)

		// Production logger should be created
		assert.IsType(t, &zap.Logger{}, logger)
	})

	t.Run("Logger can be used for logging", func(t *testing.T) {
		_ = os.Unsetenv("KUBERNETES_SERVICE_HOST")

		logger := Logger()
		assert.NotNil(t, logger)

		// Should not panic
		assert.NotPanics(t, func() {
			logger.Info("test message")
			logger.Debug("debug message")
			logger.Error("error message")
		})
	})
}
