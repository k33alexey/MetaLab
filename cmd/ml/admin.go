package main

import (
	"context"
	"fmt"
	"os"

	"github.com/k33alexey/MetaLab/internal/cli"
	"github.com/k33alexey/MetaLab/internal/localadmin"
	"github.com/k33alexey/MetaLab/internal/systemdb"
)

func resetAdministrator(ctx context.Context, login string) (cli.EmergencyCredentials, error) {
	if err := localadmin.Require(); err != nil {
		return cli.EmergencyCredentials{}, err
	}
	databaseURL := os.Getenv("ML_SYSTEM_DATABASE_URL")
	if databaseURL == "" {
		return cli.EmergencyCredentials{}, fmt.Errorf("ML_SYSTEM_DATABASE_URL is required")
	}
	database, err := systemdb.Open(ctx, databaseURL)
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
