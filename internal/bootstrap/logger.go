// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

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
