package main

import (
	"context"
	"fmt"
	"os"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/cli"
	"github.com/k33alexey/MetaLab/internal/localadmin"
	"github.com/k33alexey/MetaLab/internal/secretstore"
	"github.com/k33alexey/MetaLab/internal/systemdb"
)

func resetAdministrator(ctx context.Context, login string, configuration appconfig.Config) (cli.EmergencyCredentials, error) {
	if err := localadmin.Require(); err != nil {
		return cli.EmergencyCredentials{}, err
	}
	var database *systemdb.Database
	var err error
	if configuration.SystemDatabase != nil {
		password, secretErr := secretstore.New().Get(configuration.SystemDatabase.SecretKey)
		if secretErr != nil {
			return cli.EmergencyCredentials{}, secretErr
		}
		poolConfiguration, configErr := configuration.SystemDatabase.PoolConfig(password)
		if configErr != nil {
			return cli.EmergencyCredentials{}, configErr
		}
		database, err = systemdb.OpenConfig(ctx, poolConfiguration)
	} else if databaseURL := os.Getenv("ML_SYSTEM_DATABASE_URL"); databaseURL != "" {
		database, err = systemdb.Open(ctx, databaseURL)
	} else {
		return cli.EmergencyCredentials{}, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	if err != nil {
		return cli.EmergencyCredentials{}, err
	}
	defer database.Close()
	credentials, err := database.Administrators.EmergencyReset(ctx, login)
	if err != nil {
		return cli.EmergencyCredentials{}, err
	}
	return cli.EmergencyCredentials{
		TemporaryPassword: credentials.TemporaryPassword,
		RecoveryCodes:     credentials.RecoveryCodes,
	}, nil
}
