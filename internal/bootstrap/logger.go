package bootstrap

import (
	"go.uber.org/zap"
	"os"
)

func Logger() *zap.Logger {
	var zapLogger *zap.Logger
	var err error
	if _, ok := os.LookupEnv("KUBERNETES_SERVICE_HOST"); ok {
		zapLogger, err = zap.NewProduction()
		if err != nil {
			panic(err)
		}
	} else {
		zapLogger, err = zap.NewDevelopment()
		if err != nil {
			panic(err)
		}
	}

	return zapLogger
}
