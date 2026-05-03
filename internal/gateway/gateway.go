package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/JinBeiCN/mcp-gateway/internal/config"
	"github.com/JinBeiCN/mcp-gateway/internal/protocol"
	"github.com/JinBeiCN/mcp-gateway/internal/security"
	"github.com/JinBeiCN/mcp-gateway/pkg/mcp"
)

type Gateway struct {
	cfg             *config.Config
	backends        map[string]*BackendProxy
	authenticator   *security.Authenticator
	rbac            *security.RBAC
	rateLimiter     *security.BackendRateLimiter
	injectionDetect *security.InjectionDetector
	auditLogger     *security.AuditLogger
	httpServer      *http.Server
	mu              sync.RWMutex
	shutdownCh      chan struct{}
}

func New(cfg *config.Config) (*Gateway, error) {
	audit, err := security.NewAuditLogger(cfg.Logging.AuditFile)
	if err != nil {
		return nil, fmt.Errorf("audit logger: %w", err)
	}

	g := &Gateway{
		cfg:             cfg,
		backends:        make(map[string]*BackendProxy),
		authenticator:   security.NewAuthenticator(&cfg.Security),
		rbac:            security.NewRBAC(cfg.Backends),
		rateLimiter:     security.NewBackendRateLimiter(cfg.Backends),
		injectionDetect: security.NewInjectionDetector(&cfg.Security.Injection),
		auditLogger:     audit,
		shutdownCh:      make(chan struct{}),
	}

	return g, nil
}

func (g *Gateway) Start() error {
	// Start all backend proxies
	for i := range g.cfg.Backends {
		bcfg := &g.cfg.Backends[i]
		if !bcfg.Enabled {
			log.Printf("[gateway] skipping disabled backend: %s", bcfg.Name)
			continue
		}

		proxy, err := NewBackendProxy(bcfg)
		if err != nil {
			return fmt.Errorf("start backend %s: %w", bcfg.Name, err)
		}
		g.backends[bcfg.Name] = proxy
		log.Printf("[gateway] backend started: %s (%s %v)", bcfg.Name, bcfg.Command, bcfg.Args)
	}

	// Start HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", g.handleJSONRPC)
	mux.HandleFunc("/sse", g.handleSSE)
	mux.HandleFunc("/health", g.handleHealth)
	mux.HandleFunc("/metrics", g.handleMetrics)
	mux.HandleFunc("/api/v1/backends", g.handleListBackends)
	mux.HandleFunc("/api/v1/tools", g.handleListAllTools)

	addr := g.cfg.Server.ListenAddr
	if addr == "" {
		addr = ":9020"
	}

	g.httpServer = &http.Server{
		Addr:         addr,
		Handler:      g.loggingMiddleware(mux),
		ReadTimeout:  g.cfg.Server.ReadTimeout(),
		WriteTimeout: g.cfg.Server.WriteTimeout(),
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("[gateway] %s v%s starting on %s", g.cfg.Server.Name, g.cfg.Server.Version, addr)

	errCh := make(chan error, 1)
	go func() {
		if err := g.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		log.Printf("[gateway] received signal %v, shutting down", sig)
		return g.Shutdown()
	}
}

func (g *Gateway) Shutdown() error {
	log.Printf("[gateway] shutting down...")

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if g.httpServer != nil {
		g.httpServer.Shutdown(ctx)
	}

	// Stop all backends
	for name, proxy := range g.backends {
		log.Printf("[gateway] stopping backend: %s", name)
		proxy.Stop()
	}

	if g.auditLogger != nil {
		g.auditLogger.Close()
	}

	close(g.shutdownCh)
	log.Printf("[gateway] shutdown complete")
	return nil
}

func (g *Gateway) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate
	authResult, err := g.authenticator.Authenticate(r.Header.Get("Authorization"))
	if err != nil {
		g.auditLogger.LogSecurityBlock("unknown", err.Error(), "")
		writeJSONRPCError(w, nil, mcp.ErrUnauthorized, "authentication required")
		return
	}

	// Parse the JSON-RPC request
	var rawReq mcp.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&rawReq); err != nil {
		writeJSONRPCError(w, nil, mcp.ErrParse, "invalid JSON-RPC request")
		return
	}

	// Check rate limit
	if !g.rateLimiter.Allow(authResult.ClientID, "gateway", rawReq.Method) {
		g.auditLogger.LogSecurityBlock(authResult.ClientID, "rate limited", rawReq.Method)
		writeJSONRPCError(w, rawReq.ID, mcp.ErrRateLimited, "rate limit exceeded")
		return
	}

	// Audit log the request
	auditEvent := g.auditLogger.LogRequest(authResult.ClientID, "", rawReq.Method, rawReq.ID)
	startTime := time.Now()

	// Handle initialize specially - it returns the gateway's own capabilities
	if rawReq.Method == mcp.MethodInitialize {
		resp := g.handleInitialize(&rawReq, authResult)
		writeJSONRPCResponse(w, resp)
		g.auditLogger.LogResponse(auditEvent, resp.Error == nil, errorMsg(resp.Error), time.Since(startTime).Milliseconds())
		return
	}

	// Handle tools/list - aggregate tools from all backends
	if rawReq.Method == mcp.MethodToolsList {
		resp := g.handleToolsList(&rawReq, authResult)
		writeJSONRPCResponse(w, resp)
		g.auditLogger.LogResponse(auditEvent, resp.Error == nil, errorMsg(resp.Error), time.Since(startTime).Milliseconds())
		return
	}

	// Handle tools/call - route to specific backend
	if rawReq.Method == mcp.MethodToolsCall {
		resp := g.handleToolsCall(&rawReq, authResult)
		writeJSONRPCResponse(w, resp)
		g.auditLogger.LogResponse(auditEvent, resp.Error == nil, errorMsg(resp.Error), time.Since(startTime).Milliseconds())
		return
	}

	// For other methods, try to forward to the first available backend
	for name, proxy := range g.backends {
		result, err := proxy.SendRequest(rawReq.Method, rawReq.Params)
		if err != nil {
			writeJSONRPCError(w, rawReq.ID, mcp.ErrBackendError, fmt.Sprintf("backend %s: %v", name, err))
			g.auditLogger.LogResponse(auditEvent, false, err.Error(), time.Since(startTime).Milliseconds())
			return
		}
		resp := protocol.NewSuccessResponse(rawReq.ID, result)
		writeJSONRPCResponse(w, resp)
		g.auditLogger.LogResponse(auditEvent, true, "", time.Since(startTime).Milliseconds())
		return
	}

	writeJSONRPCError(w, rawReq.ID, mcp.ErrMethodNotFound, "no backend available")
	g.auditLogger.LogResponse(auditEvent, false, "no backend available", time.Since(startTime).Milliseconds())
}

func (g *Gateway) handleInitialize(req *mcp.JSONRPCRequest, auth *security.AuthResult) *mcp.JSONRPCResponse {
	return protocol.NewSuccessResponse(req.ID, mcp.InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: mcp.ServerCapabilities{
			Tools:     &mcp.ToolsCapability{ListChanged: false},
			Resources: &mcp.ResourcesCapability{},
			Prompts:   &mcp.PromptsCapability{},
		},
		ServerInfo: mcp.Implementation{
			Name:    g.cfg.Server.Name,
			Version: g.cfg.Server.Version,
		},
		Instructions: fmt.Sprintf("MCP Gateway protecting %d backends", len(g.backends)),
	})
}

func (g *Gateway) handleToolsList(req *mcp.JSONRPCRequest, auth *security.AuthResult) *mcp.JSONRPCResponse {
	var allTools []mcp.Tool

	for name, proxy := range g.backends {
		access := g.rbac.CheckToolAccess(name, "*", auth.Permissions)
		if !access.Allowed {
			continue
		}

		result, err := proxy.SendRequest(mcp.MethodToolsList, nil)
		if err != nil {
			log.Printf("[gateway] error listing tools from %s: %v", name, err)
			continue
		}

		var toolsResult mcp.ToolsListResult
		data, _ := json.Marshal(result)
		if err := json.Unmarshal(data, &toolsResult); err != nil {
			continue
		}

		for _, tool := range toolsResult.Tools {
			prefixedName := name + "/" + tool.Name
			tool.Name = prefixedName
			allTools = append(allTools, tool)
		}
	}

	return protocol.NewSuccessResponse(req.ID, mcp.ToolsListResult{Tools: allTools})
}

func (g *Gateway) handleToolsCall(req *mcp.JSONRPCRequest, auth *security.AuthResult) *mcp.JSONRPCResponse {
	var params mcp.ToolsCallParams
	data, _ := json.Marshal(req.Params)
	if err := json.Unmarshal(data, &params); err != nil {
		return protocol.NewErrorResponse(req.ID, mcp.ErrInvalidParams, "invalid tool call params")
	}

	// Parse backend/tool from prefixed name: "backend_name/tool_name"
	backendName, toolName := parseToolName(params.Name)
	if backendName == "" {
		return protocol.NewErrorResponse(req.ID, mcp.ErrInvalidParams, "tool name must be prefixed with backend name (backend/tool)")
	}

	proxy, ok := g.backends[backendName]
	if !ok {
		return protocol.NewErrorResponse(req.ID, mcp.ErrMethodNotFound, fmt.Sprintf("backend %q not found", backendName))
	}

	// RBAC check
	access := g.rbac.CheckToolAccess(backendName, toolName, auth.Permissions)
	if !access.Allowed {
		g.auditLogger.LogSecurityBlock(auth.ClientID, access.Reason, "tools/call")
		return protocol.NewErrorResponse(req.ID, mcp.ErrToolNotAllowed, access.Reason)
	}

	// Injection detection
	injection := g.injectionDetect.Scan(params.Arguments)
	if injection.Blocked {
		g.auditLogger.LogInjectionBlock(auth.ClientID, backendName, toolName, injection.Score)
		g.auditLogger.LogSecurityBlock(auth.ClientID, injection.Reason, "tools/call")
		return protocol.NewErrorResponse(req.ID, mcp.ErrInjectionDetected, "request blocked by content safety filter")
	}

	// Rate limit for tool
	if !g.rateLimiter.Allow(auth.ClientID, backendName, toolName) {
		g.auditLogger.LogSecurityBlock(auth.ClientID, "rate limited", "tools/call")
		return protocol.NewErrorResponse(req.ID, mcp.ErrRateLimited, "rate limit exceeded for this tool")
	}

	// Forward to backend with the original tool name (strip prefix)
	backendParams := mcp.ToolsCallParams{
		Name:      toolName,
		Arguments: params.Arguments,
	}

	result, err := proxy.SendRequest(mcp.MethodToolsCall, backendParams)
	if err != nil {
		return protocol.NewErrorResponse(req.ID, mcp.ErrBackendError, fmt.Sprintf("backend error: %v", err))
	}

	return protocol.NewSuccessResponse(req.ID, result)
}

func (g *Gateway) handleSSE(w http.ResponseWriter, r *http.Request) {
	// SSE endpoint for server→client notifications
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	fmt.Fprintf(w, "event: endpoint\ndata: /mcp\n\n")
	flusher.Flush()

	// Keep connection alive
	ctx := r.Context()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-g.shutdownCh:
			return
		}
	}
}

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	healthy := true
	backends := make(map[string]string)

	for name, proxy := range g.backends {
		if proxy.running.Load() {
			backends[name] = "healthy"
		} else {
			backends[name] = "unhealthy"
			healthy = false
		}
	}

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   map[bool]string{true: "ok", false: "degraded"}[healthy],
		"version":  g.cfg.Server.Version,
		"backends": backends,
	})
}

func (g *Gateway) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "# MCP Gateway Metrics\n")
	fmt.Fprintf(w, "mcp_gateway_backends_total %d\n", len(g.backends))

	running := 0
	for _, proxy := range g.backends {
		if proxy.running.Load() {
			running++
		}
	}
	fmt.Fprintf(w, "mcp_gateway_backends_healthy %d\n", running)
}

func (g *Gateway) handleListBackends(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type backendInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Command     string `json:"command"`
		Healthy     bool   `json:"healthy"`
	}

	var backends []backendInfo
	for name, proxy := range g.backends {
		backends = append(backends, backendInfo{
			Name:        name,
			Description: proxy.Config().Description,
			Command:     proxy.Config().Command,
			Healthy:     proxy.running.Load(),
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"backends": backends,
	})
}

func (g *Gateway) handleListAllTools(w http.ResponseWriter, r *http.Request) {
	authResult, _ := g.authenticator.Authenticate(r.Header.Get("Authorization"))

	w.Header().Set("Content-Type", "application/json")

	var allTools []mcp.Tool
	for name, proxy := range g.backends {
		result, err := proxy.SendRequest(mcp.MethodToolsList, nil)
		if err != nil {
			continue
		}
		var toolsResult mcp.ToolsListResult
		data, _ := json.Marshal(result)
		json.Unmarshal(data, &toolsResult)

		for _, tool := range toolsResult.Tools {
			tool.Name = name + "/" + tool.Name
			allTools = append(allTools, tool)
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"tools":       allTools,
		"total":       len(allTools),
		"client_id":   authResult.ClientID,
	})
}

func (g *Gateway) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[http] %s %s %s %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}

func parseToolName(prefixed string) (backend, tool string) {
	for i := 0; i < len(prefixed); i++ {
		if prefixed[i] == '/' {
			return prefixed[:i], prefixed[i+1:]
		}
	}
	return "", prefixed
}

func writeJSONRPCResponse(w http.ResponseWriter, resp *mcp.JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := protocol.NewErrorResponse(id, code, message)
	writeJSONRPCResponse(w, resp)
}

func errorMsg(err *mcp.JSONRPCError) string {
	if err == nil {
		return ""
	}
	return err.Message
}
