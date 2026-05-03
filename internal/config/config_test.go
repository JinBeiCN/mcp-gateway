package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	yaml := `
server:
  name: "test-gateway"
  version: "2.0.0"
  listen_addr: ":9999"

backends:
  - name: "echo"
    command: "echo"
    args: ["hello"]
    description: "test backend"

security:
  auth:
    enabled: true
    api_keys:
      - key: "sk-test"
        client_id: "test-client"
        permissions: ["*"]
  injection:
    enabled: false
    block_threshold: 0.5
    warn_threshold: 0.3

logging:
  level: "debug"
  format: "text"
`

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(yaml)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Server.Name != "test-gateway" {
		t.Errorf("expected test-gateway, got %s", cfg.Server.Name)
	}
	if cfg.Server.ListenAddr != ":9999" {
		t.Errorf("expected :9999, got %s", cfg.Server.ListenAddr)
	}
	if len(cfg.Backends) != 1 {
		t.Errorf("expected 1 backend, got %d", len(cfg.Backends))
	}
	if cfg.Backends[0].Name != "echo" {
		t.Errorf("expected echo, got %s", cfg.Backends[0].Name)
	}
	if !cfg.Security.Auth.Enabled {
		t.Error("expected auth enabled")
	}
	if cfg.Security.Injection.Enabled {
		t.Error("expected injection disabled")
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected debug level, got %s", cfg.Logging.Level)
	}
}

func TestDefaultValues(t *testing.T) {
	yaml := ``
	tmpfile, _ := os.CreateTemp("", "config-*.yaml")
	defer os.Remove(tmpfile.Name())
	tmpfile.Write([]byte(yaml))
	tmpfile.Close()

	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Server.Name != "mcp-gateway" {
		t.Errorf("expected mcp-gateway, got %s", cfg.Server.Name)
	}
	if cfg.Server.ListenAddr != ":9020" {
		t.Errorf("expected :9020, got %s", cfg.Server.ListenAddr)
	}
	if cfg.Server.MaxConnections != 100 {
		t.Errorf("expected 100 max connections, got %d", cfg.Server.MaxConnections)
	}
}

func TestReadTimeout(t *testing.T) {
	s := &ServerConfig{ReadTimeoutSec: 0}
	if s.ReadTimeout() == 0 {
		t.Error("expected non-zero default timeout")
	}
	s.ReadTimeoutSec = 15
	if s.ReadTimeout().Seconds() != 15 {
		t.Error("expected 15s timeout")
	}
}

func TestBackendTimeout(t *testing.T) {
	b := &BackendConfig{TimeoutSec: 0}
	if b.Timeout() == 0 {
		t.Error("expected non-zero default timeout")
	}
	b.TimeoutSec = 120
	if b.Timeout().Seconds() != 120 {
		t.Error("expected 120s timeout")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
