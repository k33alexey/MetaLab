package postgresconn

import "testing"

func TestDescriptorBuildsEscapedPoolConfiguration(t *testing.T) {
	t.Parallel()

	descriptor := Descriptor{
		Host: "db.example.test", Port: 5432, Database: "ml system", User: "ml@service",
		SSLMode: "verify-full", SecretKey: DefaultSystemSecretKey,
	}
	configuration, err := descriptor.PoolConfig("p@ss:/?#word")
	if err != nil {
		t.Fatal(err)
	}
	if configuration.ConnConfig.Host != descriptor.Host || configuration.ConnConfig.Database != descriptor.Database ||
		configuration.ConnConfig.User != descriptor.User || configuration.ConnConfig.Password != "p@ss:/?#word" {
		t.Fatalf("configuration = %+v", configuration.ConnConfig)
	}
	if descriptor.IsLocal() || !(Descriptor{Host: "127.0.0.1"}).IsLocal() {
		t.Fatal("IsLocal() returned an unexpected result")
	}
}

func TestDescriptorValidation(t *testing.T) {
	t.Parallel()

	valid := Descriptor{Host: "localhost", Port: 5432, Database: "metalab", User: "metalab", SSLMode: "disable", SecretKey: DefaultSystemSecretKey}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	invalid := []Descriptor{
		{},
		{Host: "localhost", Port: 5432, Database: "metalab", User: "metalab", SSLMode: "prefer", SecretKey: DefaultSystemSecretKey},
		{Host: "localhost", Port: 5432, Database: "metalab", User: "metalab", SSLMode: "disable"},
	}
	for _, descriptor := range invalid {
		if err := descriptor.Validate(); err == nil {
			t.Fatalf("Validate(%+v) succeeded", descriptor)
		}
	}
}

func TestDescriptorAllowsPasswordlessUnixSocketOnly(t *testing.T) {
	t.Parallel()

	socket := Descriptor{Host: "/tmp", Port: 5432, Database: "postgres", User: "postgres", SSLMode: "disable", SecretKey: DefaultSystemSecretKey}
	configuration, err := socket.PoolConfig("")
	if err != nil || configuration.ConnConfig.Host != "/tmp" || configuration.ConnConfig.Password != "" {
		t.Fatalf("configuration=%+v error=%v", configuration, err)
	}
	socket.Host = "localhost"
	if _, err := socket.PoolConfig(""); err == nil {
		t.Fatal("passwordless TCP connection was accepted")
	}
}
