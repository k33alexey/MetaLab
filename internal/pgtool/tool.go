// Package pgtool locates and runs the official PostgreSQL client utilities
// pg_dump and pg_restore: one place for their paths, their connection
// arguments, their version and their bounded diagnostics.
package pgtool

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

// DiagnosticLimit bounds the utility output an error carries.
const DiagnosticLimit = 64 << 10

var majorVersionPattern = regexp.MustCompile(`PostgreSQL\)\s+(\d+)`)

// Runner starts one utility and waits for it; output receives its stdout and stderr.
type Runner func(ctx context.Context, path string, arguments, environment []string, output io.Writer) error

// Tools are the located pg_dump and pg_restore with the runner that starts them.
type Tools struct {
	dumpPath    string
	restorePath string
	run         Runner
}

// RestoreMode says what pg_restore does with objects already in the target database.
type RestoreMode int

const (
	// IntoEmpty restores into a database that has none of the archive's objects.
	IntoEmpty RestoreMode = iota
	// ReplaceExisting drops the archive's objects that exist before creating them.
	ReplaceExisting
)

// Locate finds pg_dump and pg_restore in PATH.
func Locate() (Tools, error) {
	dumpPath, err := exec.LookPath("pg_dump")
	if err != nil {
		return Tools{}, fmt.Errorf("pg_dump was not found in PATH")
	}
	restorePath, err := exec.LookPath("pg_restore")
	if err != nil {
		return Tools{}, fmt.Errorf("pg_restore was not found in PATH")
	}
	return New(dumpPath, restorePath, runCommand), nil
}

// New builds tools from known paths and a runner; tests pass a fake runner.
func New(dumpPath, restorePath string, run Runner) Tools {
	return Tools{dumpPath: dumpPath, restorePath: restorePath, run: run}
}

// Ready reports whether both utilities and the runner are known.
func (tools Tools) Ready() bool {
	return tools.run != nil && tools.dumpPath != "" && tools.restorePath != ""
}

// Major is the major PostgreSQL version of the utilities; pg_dump and
// pg_restore of different versions are refused.
func (tools Tools) Major(ctx context.Context) (int, error) {
	dumpMajor, err := tools.major(ctx, tools.dumpPath)
	if err != nil {
		return 0, err
	}
	restoreMajor, err := tools.major(ctx, tools.restorePath)
	if err != nil {
		return 0, err
	}
	if dumpMajor != restoreMajor {
		return 0, fmt.Errorf("pg_dump major version %d differs from pg_restore %d", dumpMajor, restoreMajor)
	}
	return dumpMajor, nil
}

// Dump writes a custom-format archive of the database to archive. The
// environment comes from postgresconn.WithPasswordFile.
func (tools Tools) Dump(ctx context.Context, connection postgresconn.Descriptor, archive string, environment []string) error {
	arguments := append([]string{"--format=custom", "--no-owner", "--no-acl", "--file", archive}, connectionArguments(connection)...)
	return tools.execute(ctx, tools.dumpPath, arguments, connectionEnvironment(connection, environment))
}

// Restore restores archive into the database in one transaction, stopping at
// the first error. The environment comes from postgresconn.WithPasswordFile.
func (tools Tools) Restore(ctx context.Context, connection postgresconn.Descriptor, archive string, mode RestoreMode, environment []string) error {
	arguments := []string{"--exit-on-error", "--single-transaction", "--no-owner", "--no-acl"}
	if mode == ReplaceExisting {
		arguments = append([]string{"--clean", "--if-exists"}, arguments...)
	}
	arguments = append(append(arguments, connectionArguments(connection)...), archive)
	return tools.execute(ctx, tools.restorePath, arguments, connectionEnvironment(connection, environment))
}

func connectionArguments(connection postgresconn.Descriptor) []string {
	return []string{
		"--host", connection.Host, "--port", strconv.Itoa(int(connection.Port)),
		"--username", connection.User, "--dbname", connection.Database,
	}
}

// connectionEnvironment adds the connection's ssl_mode, which the utilities
// have no argument for; without it libpq would use its own default.
func connectionEnvironment(connection postgresconn.Descriptor, environment []string) []string {
	return append(append([]string(nil), environment...), "PGSSLMODE="+connection.SSLMode)
}

func (tools Tools) major(ctx context.Context, path string) (int, error) {
	var output bytes.Buffer
	if err := tools.run(ctx, path, []string{"--version"}, postgresconn.ToolEnvironment(""), &output); err != nil {
		return 0, fmt.Errorf("check %s version: %w", filepath.Base(path), err)
	}
	match := majorVersionPattern.FindStringSubmatch(output.String())
	if len(match) != 2 {
		return 0, fmt.Errorf("cannot parse %s version", filepath.Base(path))
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("parse %s major version: %w", filepath.Base(path), err)
	}
	return major, nil
}

func (tools Tools) execute(ctx context.Context, path string, arguments, environment []string) error {
	diagnostics := &limitedBuffer{remaining: DiagnosticLimit}
	if err := tools.run(ctx, path, arguments, environment, diagnostics); err != nil {
		message := strings.TrimSpace(diagnostics.String())
		if message == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, message)
	}
	return nil
}

func runCommand(ctx context.Context, path string, arguments, environment []string, output io.Writer) error {
	command := exec.CommandContext(ctx, path, arguments...)
	command.Env, command.Stdout, command.Stderr = environment, output, output
	return command.Run()
}

// limitedBuffer keeps the first bytes written to it and accepts the rest
// unread, so a talkative utility neither grows memory nor blocks.
type limitedBuffer struct {
	buffer    bytes.Buffer
	remaining int
}

func (buffer *limitedBuffer) Write(content []byte) (int, error) {
	length := len(content)
	if len(content) > buffer.remaining {
		content = content[:buffer.remaining]
	}
	_, _ = buffer.buffer.Write(content)
	buffer.remaining -= len(content)
	return length, nil
}

func (buffer *limitedBuffer) String() string { return buffer.buffer.String() }
