package security

import (
	"fmt"
	"strings"

	"github.com/JinBeiCN/mcp-gateway/internal/config"
)

type RBAC struct {
	backends map[string]*config.BackendConfig
}

func NewRBAC(backends []config.BackendConfig) *RBAC {
	r := &RBAC{backends: make(map[string]*config.BackendConfig)}
	for i := range backends {
		r.backends[backends[i].Name] = &backends[i]
	}
	return r
}

type AccessCheck struct {
	Allowed    bool
	Reason     string
	Tool       string
	Backend    string
}

func (r *RBAC) CheckToolAccess(backendName, toolName string, permissions []string) *AccessCheck {
	backend, ok := r.backends[backendName]
	if !ok {
		return &AccessCheck{Allowed: false, Reason: fmt.Sprintf("backend %q not found", backendName), Tool: toolName, Backend: backendName}
	}

	if !backend.Enabled {
		return &AccessCheck{Allowed: false, Reason: fmt.Sprintf("backend %q is disabled", backendName), Tool: toolName, Backend: backendName}
	}

	// Check global wildcard permission
	for _, p := range permissions {
		if p == "*" || p == "admin" {
			return r.checkBackendToolRestrictions(backend, toolName)
		}
	}

	// Check specific tool permissions
	toolPerm := fmt.Sprintf("tool:%s:%s", backendName, toolName)
	backendPerm := fmt.Sprintf("tool:%s:*", backendName)

	for _, p := range permissions {
		if p == toolPerm || p == backendPerm {
			return r.checkBackendToolRestrictions(backend, toolName)
		}
	}

	return &AccessCheck{
		Allowed: false,
		Reason:  fmt.Sprintf("no permission for tool %q on backend %q", toolName, backendName),
		Tool:    toolName,
		Backend: backendName,
	}
}

func (r *RBAC) checkBackendToolRestrictions(backend *config.BackendConfig, toolName string) *AccessCheck {
	// Check denied tools first
	for _, denied := range backend.DeniedTools {
		if matchPattern(denied, toolName) {
			return &AccessCheck{Allowed: false, Reason: fmt.Sprintf("tool %q is explicitly denied", toolName), Tool: toolName, Backend: backend.Name}
		}
	}

	// If allowed tools are specified, check against them
	if len(backend.AllowedTools) > 0 {
		for _, allowed := range backend.AllowedTools {
			if matchPattern(allowed, toolName) {
				return &AccessCheck{Allowed: true, Tool: toolName, Backend: backend.Name}
			}
		}
		return &AccessCheck{Allowed: false, Reason: fmt.Sprintf("tool %q not in allowed list", toolName), Tool: toolName, Backend: backend.Name}
	}

	return &AccessCheck{Allowed: true, Tool: toolName, Backend: backend.Name}
}

func (r *RBAC) GetAllowedTools(backendName string, permissions []string) ([]string, error) {
	backend, ok := r.backends[backendName]
	if !ok {
		return nil, fmt.Errorf("backend %q not found", backendName)
	}

	hasGlobalAccess := false
	for _, p := range permissions {
		if p == "*" || p == "admin" {
			hasGlobalAccess = true
			break
		}
	}

	if !hasGlobalAccess {
		return nil, fmt.Errorf("listing tools requires admin or wildcard permissions")
	}

	return backend.AllowedTools, nil
}

func matchPattern(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(s, prefix)
	}
	return pattern == s
}
