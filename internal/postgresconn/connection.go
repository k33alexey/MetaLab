// Package postgresconn defines non-secret PostgreSQL connection descriptors.
package postgresconn

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const DefaultSystemSecretKey = "postgres.system.password"

// Descriptor contains safe-to-persist PostgreSQL connection fields.
type Descriptor struct {
	Host      string `yaml:"host" json:"host"`
	Port      uint16 `yaml:"port" json:"port"`
	Database  string `yaml:"database" json:"database"`
	User      string `yaml:"user" json:"user"`
	SSLMode   string `yaml:"ssl_mode" json:"sslMode"`
	SecretKey string `yaml:"secret_key" json:"-"`
}

// Validate checks the non-secret connection contract.
func (descriptor Descriptor) Validate() error {
	if strings.TrimSpace(descriptor.Host) == "" || descriptor.Port == 0 {
		return fmt.Errorf("PostgreSQL host and port are required")
	}
	if strings.TrimSpace(descriptor.Database) == "" || strings.TrimSpace(descriptor.User) == "" {
		return fmt.Errorf("PostgreSQL database and user are required")
	}
	switch descriptor.SSLMode {
	case "disable", "require", "verify-ca", "verify-full":
	default:
		return fmt.Errorf("unsupported PostgreSQL ssl_mode %q", descriptor.SSLMode)
	}
	if descriptor.SecretKey == "" {
		return fmt.Errorf("PostgreSQL secret key is required")
	}
	return nil
}

// Address returns the host:port form used in diagnostics.
func (descriptor Descriptor) Address() string {
	if strings.HasPrefix(descriptor.Host, "/") {
		return descriptor.Host
	}
	return net.JoinHostPort(descriptor.Host, strconv.Itoa(int(descriptor.Port)))
}

// PoolConfig creates an in-memory pgx configuration without persisting the password.
func (descriptor Descriptor) PoolConfig(password string) (*pgxpool.Config, error) {
	if err := descriptor.Validate(); err != nil {
		return nil, err
	}
	if password == "" && !strings.HasPrefix(descriptor.Host, "/") {
		return nil, fmt.Errorf("PostgreSQL password is required")
	}
	connectionURL := &url.URL{
		Scheme: "postgres", Path: "/" + descriptor.Database,
	}
	if password == "" {
		connectionURL.User = url.User(descriptor.User)
	} else {
		connectionURL.User = url.UserPassword(descriptor.User, password)
	}
	parameters := url.Values{"sslmode": []string{descriptor.SSLMode}}
	if strings.HasPrefix(descriptor.Host, "/") {
		parameters.Set("host", descriptor.Host)
		parameters.Set("port", strconv.Itoa(int(descriptor.Port)))
	} else {
		connectionURL.Host = descriptor.Address()
	}
	connectionURL.RawQuery = parameters.Encode()
	configuration, err := pgxpool.ParseConfig(connectionURL.String())
	if err != nil {
		return nil, fmt.Errorf("build PostgreSQL connection configuration: %w", err)
	}
	return configuration, nil
}

// IsLocal reports whether the server address is loopback-only.
func (descriptor Descriptor) IsLocal() bool {
	host := strings.Trim(descriptor.Host, "[]")
	return strings.HasPrefix(host, "/") || host == "localhost" || host == "" || net.ParseIP(host).IsLoopback()
}
