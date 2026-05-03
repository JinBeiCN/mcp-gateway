package security

import (
	"testing"

	"github.com/JinBeiCN/mcp-gateway/internal/config"
)

func TestAuthenticatorDisabled(t *testing.T) {
	cfg := &config.SecurityConfig{
		Auth: config.AuthConfig{Enabled: false},
	}
	auth := NewAuthenticator(cfg)

	result, err := auth.Authenticate("")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Authenticated {
		t.Error("expected authenticated when auth disabled")
	}
	if result.ClientID != "anonymous" {
		t.Errorf("expected anonymous, got %s", result.ClientID)
	}
}

func TestAuthenticatorAPIKey(t *testing.T) {
	cfg := &config.SecurityConfig{
		Auth: config.AuthConfig{
			Enabled: true,
			APIKeys: []config.APIKeyEntry{
				{Key: "sk-test-123", ClientID: "test-client", Permissions: []string{"*"}},
			},
		},
	}
	auth := NewAuthenticator(cfg)

	result, err := auth.Authenticate("Bearer sk-test-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ClientID != "test-client" {
		t.Errorf("expected test-client, got %s", result.ClientID)
	}
	if result.AuthMethod != "api_key" {
		t.Errorf("expected api_key method, got %s", result.AuthMethod)
	}
}

func TestAuthenticatorInvalidKey(t *testing.T) {
	cfg := &config.SecurityConfig{
		Auth: config.AuthConfig{
			Enabled: true,
			APIKeys: []config.APIKeyEntry{
				{Key: "sk-test-123", ClientID: "test-client", Permissions: []string{"*"}},
			},
		},
	}
	auth := NewAuthenticator(cfg)

	_, err := auth.Authenticate("Bearer sk-wrong-key")
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestAuthenticatorNoAuthHeader(t *testing.T) {
	cfg := &config.SecurityConfig{
		Auth: config.AuthConfig{Enabled: true},
	}
	auth := NewAuthenticator(cfg)

	_, err := auth.Authenticate("")
	if err == nil {
		t.Error("expected error when auth enabled but no header")
	}
}

func TestAuthenticatorXAPIKey(t *testing.T) {
	cfg := &config.SecurityConfig{
		Auth: config.AuthConfig{
			Enabled: true,
			APIKeys: []config.APIKeyEntry{
				{Key: "sk-gateway-admin-key-001", ClientID: "admin", Permissions: []string{"*"}},
			},
		},
	}
	auth := NewAuthenticator(cfg)

	result, err := auth.Authenticate("sk-gateway-admin-key-001")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ClientID != "admin" {
		t.Errorf("expected admin, got %s", result.ClientID)
	}
}

func TestAuthenticatorPermissions(t *testing.T) {
	cfg := &config.SecurityConfig{
		Auth: config.AuthConfig{
			Enabled: true,
			APIKeys: []config.APIKeyEntry{
				{Key: "sk-readonly", ClientID: "reader", Permissions: []string{"tool:filesystem:read_file", "tool:github:search_repositories"}},
			},
		},
	}
	auth := NewAuthenticator(cfg)

	result, err := auth.Authenticate("Bearer sk-readonly")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Permissions) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(result.Permissions))
	}
}

func TestAuthenticatorConstantTimeCompare(t *testing.T) {
	cfg := &config.SecurityConfig{
		Auth: config.AuthConfig{Enabled: false},
	}
	auth := NewAuthenticator(cfg)

	if !auth.ConstantTimeCompare("abc", "abc") {
		t.Error("expected match")
	}
	if auth.ConstantTimeCompare("abc", "xyz") {
		t.Error("expected non-match")
	}
}
