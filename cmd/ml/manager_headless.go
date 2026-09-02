//go:build !desktop

package main

import (
	"context"
	"fmt"

	"github.com/k33alexey/MetaLab/internal/appconfig"
)

func runManager(context.Context, appconfig.Config) error {
	return fmt.Errorf("desktop support is not included; use a desktop MetaLab build")
}
