package bootstrap

import (
	"go.uber.org/zap"
	"os"
)

func Logger() *zap.Logger {
	var zapLogger *zap.Logger
	if _, ok := os.LookupEnv("KUBERNETES_SERVICE_HOST"); ok {
		zapLogger, _ = zap.NewProduction()
	} else {
		zapLogger, _ = zap.NewDevelopment()
	}

	return zapLogger
}
