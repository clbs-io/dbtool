package main

import (
	"context"
	"github.com/cybroslabs/hes-1-dbtool/internal/bootstrap"
	"github.com/cybroslabs/hes-1-dbtool/internal/config"
	"github.com/cybroslabs/hes-1-dbtool/internal/dbtool"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := bootstrap.Logger()
	defer func() { _ = logger.Sync() }()

	logger.Info("Starting clbs-dbtool")

	logger.Info("Loading config...")

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("Error loading config", zap.Error(err))
	}

	dbtool.Run(ctx, logger, cfg)
}
