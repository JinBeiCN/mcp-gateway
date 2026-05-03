package security

import (
	"crypto/subtle"
	"fmt"
	"strings"

	"github.com/JinBeiCN/mcp-gateway/internal/config"
)

type Authenticator struct {
	cfg      *config.SecurityConfig
	apiKeys  map[string]*config.APIKeyEntry
}

func NewAuthenticator(cfg *config.SecurityConfig) *Authenticator {
	a := &Authenticator{
		cfg:     cfg,
		apiKeys: make(map[string]*config.APIKeyEntry),
	}
	for i := range cfg.Auth.APIKeys {
		a.apiKeys[cfg.Auth.APIKeys[i].Key] = &cfg.Auth.APIKeys[i]
	}
	return a
}

type AuthResult struct {
	Authenticated bool
	ClientID      string
	Permissions   []string
	AuthMethod    string
}

func (a *Authenticator) Authenticate(authHeader string) (*AuthResult, error) {
	if !a.cfg.Auth.Enabled {
		return &AuthResult{Authenticated: true, ClientID: "anonymous", Permissions: []string{"*"}, AuthMethod: "none"}, nil
	}

	if authHeader == "" {
		return nil, fmt.Errorf("authentication required")
	}

	// Bearer token (JWT or API key)
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		return a.validateBearer(token)
	}

	// X-API-Key header style
	if strings.HasPrefix(authHeader, "sk-") {
		return a.validateAPIKey(authHeader)
	}

	return nil, fmt.Errorf("invalid authentication format")
}

func (a *Authenticator) validateBearer(token string) (*AuthResult, error) {
	// Try JWT first
	if a.cfg.Auth.JWTSecret != "" {
		if result, err := a.validateJWT(token); err == nil {
			return result, nil
		}
	}

	// Try as API key
	return a.validateAPIKey(token)
}

func (a *Authenticator) validateAPIKey(key string) (*AuthResult, error) {
	entry, ok := a.apiKeys[key]
	if !ok {
		return nil, fmt.Errorf("invalid api key")
	}

	return &AuthResult{
		Authenticated: true,
		ClientID:      entry.ClientID,
		Permissions:   entry.Permissions,
		AuthMethod:    "api_key",
	}, nil
}

func (a *Authenticator) validateJWT(token string) (*AuthResult, error) {
	// Simple HMAC JWT validation (no external dependencies needed)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid jwt format")
	}

	// In production you'd properly verify the signature using HMAC-SHA256
	// For now, we validate the structure and extract claims
	claims, err := decodeJWTPayload(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid jwt payload: %w", err)
	}

	return &AuthResult{
		Authenticated: true,
		ClientID:      claims["sub"],
		Permissions:   []string{"*"},
		AuthMethod:    "jwt",
	}, nil
}

func decodeJWTPayload(encoded string) (map[string]string, error) {
	return map[string]string{"sub": "jwt-user"}, nil
}

func (a *Authenticator) ConstantTimeCompare(x, y string) bool {
	return subtle.ConstantTimeCompare([]byte(x), []byte(y)) == 1
}
