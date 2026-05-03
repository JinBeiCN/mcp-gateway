package gateway

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JinBeiCN/mcp-gateway/internal/config"
	"github.com/JinBeiCN/mcp-gateway/pkg/mcp"
)

type BackendProxy struct {
	cfg       *config.BackendConfig
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	reader    *bufio.Reader
	mu        sync.Mutex
	running   atomic.Bool
	pending   map[interface{}]chan *mcp.JSONRPCResponse
	pendingMu sync.RWMutex
}

func NewBackendProxy(cfg *config.BackendConfig) (*BackendProxy, error) {
	bp := &BackendProxy{
		cfg:     cfg,
		pending: make(map[interface{}]chan *mcp.JSONRPCResponse),
	}

	if err := bp.start(); err != nil {
		return nil, err
	}

	return bp, nil
}

func (bp *BackendProxy) start() error {
	bp.cmd = exec.Command(bp.cfg.Command, bp.cfg.Args...)

	if len(bp.cfg.Env) > 0 {
		bp.cmd.Env = bp.cmd.Environ()
		for k, v := range bp.cfg.Env {
			bp.cmd.Env = append(bp.cmd.Env, k+"="+v)
		}
	}

	var err error
	bp.stdin, err = bp.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	bp.stdout, err = bp.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	// Capture stderr for debugging
	bp.cmd.Stderr = &stderrLogger{name: bp.cfg.Name}

	if err := bp.cmd.Start(); err != nil {
		return fmt.Errorf("start backend: %w", err)
	}

	bp.reader = bufio.NewReader(bp.stdout)
	bp.running.Store(true)

	go bp.readResponses()
	go bp.waitForExit()

	return nil
}

func (bp *BackendProxy) waitForExit() {
	if err := bp.cmd.Wait(); err != nil {
		fmt.Fprintf(&stderrLogger{name: "gateway"}, "backend %s exited: %v\n", bp.cfg.Name, err)
	}
	bp.running.Store(false)
}

func (bp *BackendProxy) readResponses() {
	for bp.running.Load() {
		line, err := bp.reader.ReadBytes('\n')
		if err != nil {
			if bp.running.Load() {
				fmt.Fprintf(&stderrLogger{name: "gateway"}, "backend %s read error: %v\n", bp.cfg.Name, err)
			}
			return
		}

		var resp mcp.JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			// Might be a notification, just log and skip
			continue
		}

		bp.pendingMu.RLock()
		ch, ok := bp.pending[resp.ID]
		bp.pendingMu.RUnlock()

		if ok {
			select {
			case ch <- &resp:
			default:
			}
		}
	}
}

func (bp *BackendProxy) SendRequest(method string, params interface{}) (interface{}, error) {
	id := fmt.Sprintf("req-%d", time.Now().UnixNano())

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	respCh := make(chan *mcp.JSONRPCResponse, 1)

	bp.pendingMu.Lock()
	bp.pending[id] = respCh
	bp.pendingMu.Unlock()

	defer func() {
		bp.pendingMu.Lock()
		delete(bp.pending, id)
		bp.pendingMu.Unlock()
	}()

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	bp.mu.Lock()
	_, err = bp.stdin.Write(data)
	bp.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write to backend: %w", err)
	}

	timeout := bp.cfg.Timeout()
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("backend error [%d]: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("backend timeout after %v", timeout)
	}
}

func (bp *BackendProxy) ForwardRequest(raw json.RawMessage) (*mcp.JSONRPCResponse, error) {
	var req mcp.JSONRPCRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("parse request: %w", err)
	}

	id := req.ID
	respCh := make(chan *mcp.JSONRPCResponse, 1)

	bp.pendingMu.Lock()
	bp.pending[id] = respCh
	bp.pendingMu.Unlock()

	defer func() {
		bp.pendingMu.Lock()
		delete(bp.pending, id)
		bp.pendingMu.Unlock()
	}()

	bp.mu.Lock()
	_, err := bp.stdin.Write(raw)
	bp.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write to backend: %w", err)
	}

	timeout := bp.cfg.Timeout()
	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("backend timeout after %v", timeout)
	}
}

func (bp *BackendProxy) Stop() error {
	bp.running.Store(false)

	if bp.stdin != nil {
		bp.stdin.Close()
	}

	if bp.cmd != nil && bp.cmd.Process != nil {
		bp.cmd.Process.Kill()
	}

	return nil
}

func (bp *BackendProxy) Name() string {
	return bp.cfg.Name
}

func (bp *BackendProxy) Config() *config.BackendConfig {
	return bp.cfg
}

type stderrLogger struct {
	name string
}

func (s *stderrLogger) Write(p []byte) (n int, err error) {
	fmt.Printf("[%s:stderr] %s", s.name, string(p))
	return len(p), nil
}
