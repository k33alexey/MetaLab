// Package postgresadmin validates PostgreSQL and provisions least-privilege ML databases.
package postgresadmin

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/pbkdf2"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

const (
	MinimumServerMajor = 16
	MaximumServerMajor = 18
	defaultIterations  = 4096
)

var simpleName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// Check describes a validated PostgreSQL server and administrative privileges.
type Check struct {
	VersionNumber int    `json:"versionNumber"`
	Version       string `json:"version"`
	Encoding      string `json:"encoding"`
	InRecovery    bool   `json:"inRecovery"`
	Superuser     bool   `json:"superuser"`
	CanCreateDB   bool   `json:"canCreateDatabase"`
	CanCreateRole bool   `json:"canCreateRole"`
}

// CanProvision reports whether the connection can create an isolated database and role.
func (check Check) CanProvision() bool {
	return !check.InRecovery && (check.Superuser || check.CanCreateDB) && (check.Superuser || check.CanCreateRole)
}

// Provisioned contains the safe descriptor and one-time generated password.
type Provisioned struct {
	Connection postgresconn.Descriptor
	Password   string
}

// Validate connects, checks compatibility and reads the current role capabilities.
func Validate(ctx context.Context, descriptor postgresconn.Descriptor, password string) (Check, error) {
	pool, err := open(ctx, descriptor, password)
	if err != nil {
		return Check{}, err
	}
	defer pool.Close()
	var check Check
	err = pool.QueryRow(ctx, `
SELECT current_setting('server_version_num')::integer,
       current_setting('server_version'), current_setting('server_encoding'), pg_is_in_recovery(),
       roles.rolsuper, roles.rolcreatedb, roles.rolcreaterole
FROM pg_catalog.pg_roles AS roles WHERE roles.rolname = current_user`).Scan(
		&check.VersionNumber, &check.Version, &check.Encoding, &check.InRecovery,
		&check.Superuser, &check.CanCreateDB, &check.CanCreateRole,
	)
	if err != nil {
		return Check{}, fmt.Errorf("inspect PostgreSQL server: %w", err)
	}
	major := check.VersionNumber / 10000
	if major < MinimumServerMajor || major > MaximumServerMajor {
		return Check{}, fmt.Errorf("unsupported PostgreSQL major version %d; supported versions are %d-%d", major, MinimumServerMajor, MaximumServerMajor)
	}
	if check.Encoding != "UTF8" {
		return Check{}, fmt.Errorf("PostgreSQL server encoding must be UTF8, got %s", check.Encoding)
	}
	return check, nil
}

// Provision creates a fresh technical role and database, then validates their connection.
func Provision(ctx context.Context, administrator postgresconn.Descriptor, administratorPassword, databaseName, roleName string) (Provisioned, error) {
	if !simpleName.MatchString(databaseName) || !simpleName.MatchString(roleName) {
		return Provisioned{}, fmt.Errorf("PostgreSQL database and role names must match %s", simpleName.String())
	}
	check, err := Validate(ctx, administrator, administratorPassword)
	if err != nil {
		return Provisioned{}, err
	}
	if !check.CanProvision() {
		return Provisioned{}, fmt.Errorf("PostgreSQL account cannot create both databases and roles")
	}
	pool, err := open(ctx, administrator, administratorPassword)
	if err != nil {
		return Provisioned{}, err
	}
	defer pool.Close()
	if err := ensureNamesAvailable(ctx, pool, databaseName, roleName); err != nil {
		return Provisioned{}, err
	}
	password, err := generatePassword()
	if err != nil {
		return Provisioned{}, err
	}
	iterations := readSCRAMIterations(ctx, pool)
	verifier, err := scramVerifier(password, iterations)
	if err != nil {
		return Provisioned{}, err
	}
	roleIdentifier := pgx.Identifier{roleName}.Sanitize()
	databaseIdentifier := pgx.Identifier{databaseName}.Sanitize()
	if _, err := pool.Exec(ctx, fmt.Sprintf(
		"CREATE ROLE %s LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION NOBYPASSRLS PASSWORD '%s'",
		roleIdentifier, verifier,
	)); err != nil {
		return Provisioned{}, fmt.Errorf("create PostgreSQL technical role: %w", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(
		"CREATE DATABASE %s OWNER %s ENCODING 'UTF8' TEMPLATE template0",
		databaseIdentifier, roleIdentifier,
	)); err != nil {
		cause := fmt.Errorf("create PostgreSQL database: %w", err)
		cleanupCtx, cancel := rollbackContext(ctx)
		defer cancel()
		if _, cleanupErr := pool.Exec(cleanupCtx, "DROP ROLE IF EXISTS "+roleIdentifier); cleanupErr != nil {
			cause = errors.Join(cause, fmt.Errorf("drop provisioned PostgreSQL role: %w", cleanupErr))
		}
		return Provisioned{}, cause
	}
	connection := postgresconn.Descriptor{
		Host: administrator.Host, Port: administrator.Port, Database: databaseName, User: roleName,
		SSLMode: administrator.SSLMode, SecretKey: postgresconn.DefaultSystemSecretKey,
	}
	technicalPool, err := open(ctx, connection, password)
	if err != nil {
		cause := fmt.Errorf("validate PostgreSQL technical connection: %w", err)
		cleanupCtx, cancel := rollbackContext(ctx)
		defer cancel()
		return Provisioned{}, errors.Join(cause, cleanupProvisioned(cleanupCtx, pool, databaseIdentifier, roleIdentifier))
	}
	technicalPool.Close()
	return Provisioned{Connection: connection, Password: password}, nil
}

// RollbackProvisioned removes an exact database/role pair created by a failed setup.
// It refuses removal when the role is not the database owner.
func RollbackProvisioned(ctx context.Context, administrator postgresconn.Descriptor, administratorPassword string, provisioned Provisioned) error {
	databaseName, roleName := provisioned.Connection.Database, provisioned.Connection.User
	if !simpleName.MatchString(databaseName) || !simpleName.MatchString(roleName) {
		return fmt.Errorf("refuse rollback of unsafe PostgreSQL names")
	}
	pool, err := open(ctx, administrator, administratorPassword)
	if err != nil {
		return err
	}
	defer pool.Close()
	var owner string
	err = pool.QueryRow(ctx, `
SELECT roles.rolname
FROM pg_catalog.pg_database AS databases
JOIN pg_catalog.pg_roles AS roles ON roles.oid = databases.datdba
WHERE databases.datname = $1`, databaseName).Scan(&owner)
	if err != nil {
		return fmt.Errorf("verify PostgreSQL rollback ownership: %w", err)
	}
	if owner != roleName {
		return fmt.Errorf("refuse rollback: role %q does not own database %q", roleName, databaseName)
	}
	return cleanupProvisioned(ctx, pool, pgx.Identifier{databaseName}.Sanitize(), pgx.Identifier{roleName}.Sanitize())
}

func open(ctx context.Context, descriptor postgresconn.Descriptor, password string) (*pgxpool.Pool, error) {
	configuration, err := descriptor.PoolConfig(password)
	if err != nil {
		return nil, err
	}
	configuration.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, configuration)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to PostgreSQL at %s: %w", descriptor.Address(), err)
	}
	return pool, nil
}

func ensureNamesAvailable(ctx context.Context, pool *pgxpool.Pool, databaseName, roleName string) error {
	var databaseExists, roleExists bool
	if err := pool.QueryRow(ctx, `
SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_database WHERE datname = $1),
       EXISTS(SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = $2)`, databaseName, roleName).Scan(&databaseExists, &roleExists); err != nil {
		return fmt.Errorf("check PostgreSQL names: %w", err)
	}
	if databaseExists {
		return fmt.Errorf("PostgreSQL database %q already exists", databaseName)
	}
	if roleExists {
		return fmt.Errorf("PostgreSQL role %q already exists", roleName)
	}
	return nil
}

func generatePassword() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate PostgreSQL technical password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(secret), nil
}

func readSCRAMIterations(ctx context.Context, pool *pgxpool.Pool) int {
	var value string
	if err := pool.QueryRow(ctx, "SELECT current_setting('scram_iterations', true)").Scan(&value); err != nil {
		return defaultIterations
	}
	iterations, err := strconv.Atoi(value)
	if err != nil || iterations < defaultIterations || iterations > 1_000_000 {
		return defaultIterations
	}
	return iterations
}

func scramVerifier(password string, iterations int) (string, error) {
	if password == "" || iterations < defaultIterations || iterations > 1_000_000 {
		return "", fmt.Errorf("invalid SCRAM-SHA-256 verifier parameters")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate PostgreSQL SCRAM salt: %w", err)
	}
	saltedPassword := pbkdf2.Key([]byte(password), salt, iterations, sha256.Size, sha256.New)
	clientKey := hmacSHA256(saltedPassword, []byte("Client Key"))
	storedKey := sha256.Sum256(clientKey)
	serverKey := hmacSHA256(saltedPassword, []byte("Server Key"))
	return strings.Join([]string{
		"SCRAM-SHA-256$" + strconv.Itoa(iterations) + ":" + base64.StdEncoding.EncodeToString(salt),
		base64.StdEncoding.EncodeToString(storedKey[:]) + ":" + base64.StdEncoding.EncodeToString(serverKey),
	}, "$"), nil
}

func hmacSHA256(key, message []byte) []byte {
	digest := hmac.New(sha256.New, key)
	_, _ = digest.Write(message)
	return digest.Sum(nil)
}

func cleanupProvisioned(ctx context.Context, pool *pgxpool.Pool, databaseIdentifier, roleIdentifier string) error {
	if _, err := pool.Exec(ctx, "DROP DATABASE IF EXISTS "+databaseIdentifier+" WITH (FORCE)"); err != nil {
		return fmt.Errorf("drop provisioned PostgreSQL database: %w", err)
	}
	if _, err := pool.Exec(ctx, "DROP ROLE IF EXISTS "+roleIdentifier); err != nil {
		return fmt.Errorf("drop provisioned PostgreSQL role: %w", err)
	}
	return nil
}

func rollbackContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), 15*time.Second)
}
