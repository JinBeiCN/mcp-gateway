package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JinBeiCN/mcp-gateway/internal/config"
	"github.com/JinBeiCN/mcp-gateway/internal/security"
	"github.com/JinBeiCN/mcp-gateway/pkg/mcp"
)

func TestGatewayNew(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Name:       "test-gateway",
			Version:    "1.0.0",
			ListenAddr: ":0",
		},
		Security: config.SecurityConfig{
			Injection: config.InjectionConfig{Enabled: false},
		},
	}

	gw, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create gateway: %v", err)
	}
	if gw == nil {
		t.Fatal("expected non-nil gateway")
	}
}

func TestParseToolName(t *testing.T) {
	tests := []struct {
		input   string
		backend string
		tool    string
	}{
		{"filesystem/read_file", "filesystem", "read_file"},
		{"github/search_repos", "github", "search_repos"},
		{"single", "", "single"},
		{"a/b/c", "a", "b/c"},
		{"", "", ""},
	}

	for _, tt := range tests {
		backend, tool := parseToolName(tt.input)
		if backend != tt.backend || tool != tt.tool {
			t.Errorf("parseToolName(%q): expected (%q, %q), got (%q, %q)",
				tt.input, tt.backend, tt.tool, backend, tool)
		}
	}
}

func TestHealthEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Name:       "test-gateway",
			Version:    "1.0.0",
			ListenAddr: ":0",
		},
		Security: config.SecurityConfig{
			Injection: config.InjectionConfig{Enabled: false},
		},
	}

	gw, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	gw.handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
}

func TestMetricsEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Name:       "test-gateway",
			Version:    "1.0.0",
			ListenAddr: ":0",
		},
		Security: config.SecurityConfig{
			Injection: config.InjectionConfig{Enabled: false},
		},
	}

	gw, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()

	gw.handleMetrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestInitializeHandler(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Name: "test-gw", Version: "2.0.0"},
		Security: config.SecurityConfig{
			Injection: config.InjectionConfig{Enabled: false},
		},
	}

	gw := &Gateway{cfg: cfg}

	req := &mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "initialize",
		Params: mcp.InitializeParams{
			ProtocolVersion: "2024-11-05",
			ClientInfo:      mcp.Implementation{Name: "test-client", Version: "1.0"},
		},
	}

	resp := gw.handleInitialize(req, &security.AuthResult{
		Authenticated: true,
		ClientID:      "test-client",
		Permissions:   []string{"*"},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	data, _ := json.Marshal(resp.Result)
	var initResult mcp.InitializeResult
	json.Unmarshal(data, &initResult)

	if initResult.ProtocolVersion != "2024-11-05" {
		t.Errorf("expected protocol version 2024-11-05, got %s", initResult.ProtocolVersion)
	}
}

func TestToolsCallInvalidToolName(t *testing.T) {
	cfg := &config.Config{
		Security: config.SecurityConfig{
			Injection: config.InjectionConfig{Enabled: false},
		},
	}

	gw := &Gateway{cfg: cfg}

	req := &mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "tools/call",
		Params:  mcp.ToolsCallParams{Name: "tool_without_backend_prefix", Arguments: map[string]interface{}{}},
	}

	resp := gw.handleToolsCall(req, &security.AuthResult{
		Authenticated: true,
		ClientID:      "test-client",
		Permissions:   []string{"*"},
	})
	if resp.Error == nil {
		t.Fatal("expected error for tool without backend prefix")
	}
	if resp.Error.Code != mcp.ErrInvalidParams {
		t.Errorf("expected ErrInvalidParams, got %d", resp.Error.Code)
	}
}

func TestToolsCallNonexistentBackend(t *testing.T) {
	cfg := &config.Config{
		Security: config.SecurityConfig{
			Injection: config.InjectionConfig{Enabled: false},
		},
	}

	audit, _ := security.NewAuditLogger("")

	gw := &Gateway{
		cfg:         cfg,
		backends:    make(map[string]*BackendProxy),
		auditLogger: audit,
	}

	req := &mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "tools/call",
		Params:  mcp.ToolsCallParams{Name: "nonexistent/tool", Arguments: map[string]interface{}{}},
	}

	resp := gw.handleToolsCall(req, &security.AuthResult{
		Authenticated: true,
		ClientID:      "test-client",
		Permissions:   []string{"*"},
	})
	if resp.Error == nil {
		t.Fatal("expected error for nonexistent backend")
	}
}

func TestJSONRPCErrorWriting(t *testing.T) {
	var buf bytes.Buffer
	w := &testResponseWriter{header: make(http.Header), body: &buf}

	writeJSONRPCError(w, "test-id", mcp.ErrUnauthorized, "unauthorized")

	var resp mcp.JSONRPCResponse
	json.NewDecoder(&buf).Decode(&resp)

	if resp.Error.Code != mcp.ErrUnauthorized {
		t.Errorf("expected code %d, got %d", mcp.ErrUnauthorized, resp.Error.Code)
	}
}

func TestHandleListBackends(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{Name: "test-backend", Description: "test", Command: "echo"},
		},
	}

	gw := &Gateway{
		cfg:      cfg,
		backends: make(map[string]*BackendProxy),
	}

	req := httptest.NewRequest("GET", "/api/v1/backends", nil)
	rec := httptest.NewRecorder()

	gw.handleListBackends(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuditLoggerIntegration(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Name: "test", Version: "1.0"},
		Security: config.SecurityConfig{
			Injection: config.InjectionConfig{Enabled: false},
		},
	}

	gw, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	event := gw.auditLogger.LogRequest("test-client", "test-backend", "tools/call", "req-1")
	if event.ClientID != "test-client" {
		t.Errorf("expected test-client, got %s", event.ClientID)
	}

	gw.auditLogger.LogResponse(event, true, "", 42)
	if !event.Success {
		t.Error("expected success")
	}
	if event.DurationMs != 42 {
		t.Errorf("expected 42ms, got %d", event.DurationMs)
	}
}

// Helpers

type testResponseWriter struct {
	header http.Header
	body   *bytes.Buffer
	code   int
}

func (w *testResponseWriter) Header() http.Header        { return w.header }
func (w *testResponseWriter) Write(b []byte) (int, error) { return w.body.Write(b) }
func (w *testResponseWriter) WriteHeader(code int)        { w.code = code }
