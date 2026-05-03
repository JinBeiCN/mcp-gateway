package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig    `yaml:"server"`
	Backends []BackendConfig `yaml:"backends"`
	Security SecurityConfig  `yaml:"security"`
	Logging  LoggingConfig   `yaml:"logging"`
}

type ServerConfig struct {
	Name            string `yaml:"name"`
	Version         string `yaml:"version"`
	ListenAddr      string `yaml:"listen_addr"`
	MaxConnections  int    `yaml:"max_connections"`
	ReadTimeoutSec  int    `yaml:"read_timeout_sec"`
	WriteTimeoutSec int    `yaml:"write_timeout_sec"`
}

func (s *ServerConfig) ReadTimeout() time.Duration {
	if s.ReadTimeoutSec <= 0 {
		return 30 * time.Second
	}
	return time.Duration(s.ReadTimeoutSec) * time.Second
}

func (s *ServerConfig) WriteTimeout() time.Duration {
	if s.WriteTimeoutSec <= 0 {
		return 60 * time.Second
	}
	return time.Duration(s.WriteTimeoutSec) * time.Second
}

type BackendConfig struct {
	Name        string            `yaml:"name"`
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args"`
	Env         map[string]string `yaml:"env"`
	Description string            `yaml:"description"`
	Enabled     bool              `yaml:"enabled"`
	AllowedTools []string         `yaml:"allowed_tools"`
	DeniedTools  []string         `yaml:"denied_tools"`
	RateLimit    *RateLimitConfig `yaml:"rate_limit"`
	TimeoutSec   int              `yaml:"timeout_sec"`
}

func (b *BackendConfig) Timeout() time.Duration {
	if b.TimeoutSec <= 0 {
		return 60 * time.Second
	}
	return time.Duration(b.TimeoutSec) * time.Second
}

type RateLimitConfig struct {
	RequestsPerSec  int `yaml:"requests_per_sec"`
	BurstSize       int `yaml:"burst_size"`
}

type SecurityConfig struct {
	Auth      AuthConfig      `yaml:"auth"`
	Injection InjectionConfig `yaml:"injection"`
}

type AuthConfig struct {
	Enabled   bool              `yaml:"enabled"`
	APIKeys   []APIKeyEntry     `yaml:"api_keys"`
	JWTSecret string            `yaml:"jwt_secret"`
}

type APIKeyEntry struct {
	Key         string   `yaml:"key"`
	ClientID    string   `yaml:"client_id"`
	Permissions []string `yaml:"permissions"`
	RateLimit   *RateLimitConfig `yaml:"rate_limit"`
}

type InjectionConfig struct {
	Enabled        bool     `yaml:"enabled"`
	BlockThreshold float64  `yaml:"block_threshold"`
	WarnThreshold  float64  `yaml:"warn_threshold"`
	BlockedPatterns []string `yaml:"blocked_patterns"`
}

type LoggingConfig struct {
	Level      string `yaml:"level"`
	AuditFile  string `yaml:"audit_file"`
	Format     string `yaml:"format"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{
		Server: ServerConfig{
			Name:            "mcp-gateway",
			Version:         "1.0.0",
			ListenAddr:      ":9020",
			MaxConnections:  100,
			ReadTimeoutSec:  30,
			WriteTimeoutSec: 60,
		},
		Security: SecurityConfig{
			Injection: InjectionConfig{
				Enabled:        true,
				BlockThreshold: 0.8,
				WarnThreshold:  0.5,
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	for i := range cfg.Backends {
		if !cfg.Backends[i].Enabled {
			cfg.Backends[i].Enabled = true
		}
	}

	return cfg, nil
}
