package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

func TestLoadConfigUsesBackendEnv(t *testing.T) {
	sample, err := os.ReadFile("../../.env.sample")
	if err != nil {
		t.Fatal(err)
	}
	variables, err := godotenv.Read("../../.env.sample")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "TASKER_") {
			variables[key] = ""
		}
	}
	for key := range variables {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}

	root := t.TempDir()
	backend := filepath.Join(root, "apps", "backend")
	if err := os.MkdirAll(backend, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		filepath.Join(root, "compose.yaml"): []byte("services: {}\n"),
		filepath.Join(root, ".env"): []byte(strings.Replace(
			string(sample), `TASKER_PRIMARY.ENV="local"`, `TASKER_PRIMARY.ENV="root"`, 1,
		)),
		filepath.Join(backend, "go.mod"): []byte("module example.com/backend\n"),
		filepath.Join(backend, ".env"):   sample,
	}
	for path, content := range files {
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Chdir(backend)
	t.Setenv("TASKER_SERVER.PORT", "9090")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Primary.Env != "local" {
		t.Errorf("environment = %q, want local", cfg.Primary.Env)
	}
	if cfg.Server.Port != "9090" {
		t.Errorf("port = %q, want process environment value 9090", cfg.Server.Port)
	}
	if cfg.Database.Password != "postgres" {
		t.Error("database password does not match the Compose default")
	}
}
