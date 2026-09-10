package main

import (
	"context"
	"fmt"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/cli"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/secretstore"
	"github.com/k33alexey/MetaLab/internal/testsuite"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func runProjectTests(ctx context.Context, configuration appconfig.Config, request cli.TestRequest) (testsuite.Report, error) {
	if err := testsuite.EnsureConflictFree(ctx, request.ProjectPath); err != nil {
		return testsuite.Report{}, err
	}
	databaseID, err := uuid.Parse(request.DatabaseID)
	if err != nil {
		return testsuite.Report{}, fmt.Errorf("invalid database identifier: %w", err)
	}
	program, err := testsuite.CompileProject(request.ProjectPath)
	if err != nil {
		return testsuite.Report{}, err
	}
	catalog, err := metadata.Load(request.ProjectPath)
	if err != nil {
		return testsuite.Report{}, err
	}
	platformRuntime := platform.New(ctx, configuration, secretstore.New())
	defer platformRuntime.Close()
	pool, _, err := platformRuntime.OpenDebugDatabase(ctx, databaseID)
	if err != nil {
		return testsuite.Report{}, err
	}
	defer pool.Close()
	runtime, err := metadata.NewApplicationRuntime(pool, catalog, nil)
	if err != nil {
		return testsuite.Report{}, err
	}
	return testsuite.RunWithOptions(ctx, program, runtime, testsuite.RunOptions{
		Selection: request.Selection,
		Role:      request.Role,
	})
}
