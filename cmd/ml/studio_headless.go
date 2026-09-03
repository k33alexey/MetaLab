//go:build !desktop

package main

import (
	"context"
	"fmt"

	"github.com/k33alexey/MetaLab/internal/appconfig"
)

func runStudio(context.Context, appconfig.Config, string, string) error {
	return fmt.Errorf("desktop support is not included; use a desktop MetaLab build")
}
