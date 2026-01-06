// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/clbs-io/dbtool/internal/bootstrap"
	"github.com/clbs-io/dbtool/internal/config"
	"github.com/clbs-io/dbtool/internal/dbtool"
	"go.uber.org/zap"
)

var (
	Version = "dev"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	zap_logger := bootstrap.Logger()
	defer func() { _ = zap_logger.Sync() }()
	logger := zap_logger.Sugar()

	logger.Infof("Starting clbs-dbtool %v...", Version)

	logger.Info("Loading config...")

	cfg, err := config.LoadConfig(Version)
	if err != nil {
		logger.Fatal("Error loading config", zap.Error(err))
	}

	dbtool.Run(ctx, zap_logger, cfg)
}
