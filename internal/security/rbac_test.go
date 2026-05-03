package security

import (
	"testing"

	"github.com/JinBeiCN/mcp-gateway/internal/config"
)

func TestRBACWildcardAccess(t *testing.T) {
	backends := []config.BackendConfig{
		{Name: "test-backend", Enabled: true},
	}
	rbac := NewRBAC(backends)

	result := rbac.CheckToolAccess("test-backend", "read_file", []string{"*"})
	if !result.Allowed {
		t.Errorf("expected allowed with wildcard, got denied: %s", result.Reason)
	}
}

func TestRBACAdminAccess(t *testing.T) {
	backends := []config.BackendConfig{
		{Name: "test-backend", Enabled: true},
	}
	rbac := NewRBAC(backends)

	result := rbac.CheckToolAccess("test-backend", "read_file", []string{"admin"})
	if !result.Allowed {
		t.Errorf("expected allowed with admin, got denied: %s", result.Reason)
	}
}

func TestRBACSpecificToolPermission(t *testing.T) {
	backends := []config.BackendConfig{
		{Name: "test-backend", Enabled: true},
	}
	rbac := NewRBAC(backends)

	result := rbac.CheckToolAccess("test-backend", "read_file", []string{"tool:test-backend:read_file"})
	if !result.Allowed {
		t.Errorf("expected allowed with specific permission, got denied: %s", result.Reason)
	}
}

func TestRBACBackendWildcardPermission(t *testing.T) {
	backends := []config.BackendConfig{
		{Name: "test-backend", Enabled: true},
	}
	rbac := NewRBAC(backends)

	result := rbac.CheckToolAccess("test-backend", "any_tool", []string{"tool:test-backend:*"})
	if !result.Allowed {
		t.Errorf("expected allowed with backend wildcard, got denied: %s", result.Reason)
	}
}

func TestRBACDeniedAccess(t *testing.T) {
	backends := []config.BackendConfig{
		{Name: "test-backend", Enabled: true},
	}
	rbac := NewRBAC(backends)

	result := rbac.CheckToolAccess("test-backend", "restricted_tool", []string{"tool:other-backend:*"})
	if result.Allowed {
		t.Error("expected denied when no matching permission")
	}
}

func TestRBACDisabledBackend(t *testing.T) {
	backends := []config.BackendConfig{
		{Name: "test-backend", Enabled: false},
	}
	rbac := NewRBAC(backends)

	result := rbac.CheckToolAccess("test-backend", "some_tool", []string{"*"})
	if result.Allowed {
		t.Error("expected denied for disabled backend")
	}
}

func TestRBACNonExistentBackend(t *testing.T) {
	rbac := NewRBAC(nil)

	result := rbac.CheckToolAccess("nonexistent", "tool", []string{"*"})
	if result.Allowed {
		t.Error("expected denied for nonexistent backend")
	}
}

func TestRBACDeniedTools(t *testing.T) {
	backends := []config.BackendConfig{
		{Name: "test-backend", Enabled: true, DeniedTools: []string{"dangerous_tool"}},
	}
	rbac := NewRBAC(backends)

	result := rbac.CheckToolAccess("test-backend", "dangerous_tool", []string{"*"})
	if result.Allowed {
		t.Error("expected denied for tool in denied list")
	}
}

func TestRBACDeniedToolsWildcard(t *testing.T) {
	backends := []config.BackendConfig{
		{Name: "test-backend", Enabled: true, DeniedTools: []string{"admin_*"}},
	}
	rbac := NewRBAC(backends)

	tests := []struct {
		tool    string
		allowed bool
	}{
		{"admin_delete", false},
		{"admin_create", false},
		{"read_file", true},
		{"normal_tool", true},
	}

	for _, tt := range tests {
		result := rbac.CheckToolAccess("test-backend", tt.tool, []string{"*"})
		if result.Allowed != tt.allowed {
			t.Errorf("tool %q: expected allowed=%v, got allowed=%v", tt.tool, tt.allowed, result.Allowed)
		}
	}
}

func TestRBACAllowedToolsRestriction(t *testing.T) {
	backends := []config.BackendConfig{
		{Name: "test-backend", Enabled: true, AllowedTools: []string{"read_file", "list_*"}},
	}
	rbac := NewRBAC(backends)

	tests := []struct {
		tool    string
		allowed bool
	}{
		{"read_file", true},
		{"list_files", true},
		{"list_issues", true},
		{"delete_file", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		result := rbac.CheckToolAccess("test-backend", tt.tool, []string{"*"})
		if result.Allowed != tt.allowed {
			t.Errorf("tool %q: expected allowed=%v, got allowed=%v", tt.tool, tt.allowed, result.Allowed)
		}
	}
}

func TestRBACPatternMatch(t *testing.T) {
	tests := []struct {
		pattern string
		s       string
		match   bool
	}{
		{"*", "anything", true},
		{"admin_*", "admin_delete", true},
		{"admin_*", "user_delete", false},
		{"exact_tool", "exact_tool", true},
		{"exact_tool", "other_tool", false},
		{"prefix*", "prefix", true},
		{"prefix*", "prefix_suffix", true},
	}

	for _, tt := range tests {
		result := matchPattern(tt.pattern, tt.s)
		if result != tt.match {
			t.Errorf("matchPattern(%q, %q): expected %v, got %v", tt.pattern, tt.s, tt.match, result)
		}
	}
}
