package security

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type AuditEvent struct {
	Timestamp  string      `json:"timestamp"`
	EventType  string      `json:"event_type"`
	ClientID   string      `json:"client_id,omitempty"`
	Backend    string      `json:"backend,omitempty"`
	Tool       string      `json:"tool,omitempty"`
	Method     string      `json:"method,omitempty"`
	RequestID  interface{} `json:"request_id,omitempty"`
	Success    bool        `json:"success"`
	ErrorMsg   string      `json:"error,omitempty"`
	DurationMs int64       `json:"duration_ms,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type AuditLogger struct {
	mu     sync.Mutex
	writer io.Writer
	file   *os.File
}

func NewAuditLogger(filePath string) (*AuditLogger, error) {
	a := &AuditLogger{}

	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("open audit file: %w", err)
		}
		a.file = f
		a.writer = f
	} else {
		a.writer = os.Stderr
	}

	return a, nil
}

func (a *AuditLogger) Log(event *AuditEvent) {
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	data, _ := json.Marshal(event)
	fmt.Fprintf(a.writer, "%s\n", data)
}

func (a *AuditLogger) LogRequest(clientID, backend, method string, id interface{}) *AuditEvent {
	event := &AuditEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		EventType: "request",
		ClientID:  clientID,
		Backend:   backend,
		Method:    method,
		RequestID: id,
	}
	a.Log(event)
	return event
}

func (a *AuditLogger) LogResponse(event *AuditEvent, success bool, errMsg string, durationMs int64) {
	event.EventType = "response"
	event.Success = success
	event.ErrorMsg = errMsg
	event.DurationMs = durationMs
	a.Log(event)
}

func (a *AuditLogger) LogSecurityBlock(clientID, reason, method string) {
	a.Log(&AuditEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		EventType: "security_block",
		ClientID:  clientID,
		Method:    method,
		Success:   false,
		ErrorMsg:  reason,
	})
}

func (a *AuditLogger) LogInjectionBlock(clientID, backend, tool string, score float64) {
	a.Log(&AuditEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		EventType: "injection_block",
		ClientID:  clientID,
		Backend:   backend,
		Tool:      tool,
		Metadata:  map[string]interface{}{"score": score},
	})
}

func (a *AuditLogger) Close() error {
	if a.file != nil {
		return a.file.Close()
	}
	return nil
}
