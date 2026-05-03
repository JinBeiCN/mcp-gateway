package protocol

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/JinBeiCN/mcp-gateway/pkg/mcp"
)

func TestCodecReadWrite(t *testing.T) {
	var buf bytes.Buffer
	codec := NewCodec(&buf, &buf)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "tools/list",
	}
	if err := codec.WriteMessage(req); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg, err := codec.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if msg.Type != TypeRequest {
		t.Errorf("expected request, got type %d", msg.Type)
	}
	if msg.Request.Method != "tools/list" {
		t.Errorf("expected tools/list, got %s", msg.Request.Method)
	}
	if msg.Request.ID.(string) != "1" {
		t.Errorf("expected id 1, got %v", msg.Request.ID)
	}
}

func TestCodecNotification(t *testing.T) {
	var buf bytes.Buffer
	codec := NewCodec(&buf, &buf)

	notif := mcp.JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	if err := codec.WriteMessage(notif); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg, err := codec.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if msg.Type != TypeRequest {
		t.Logf("notification type: %d", msg.Type)
	}
}

func TestCodecResponse(t *testing.T) {
	var buf bytes.Buffer
	codec := NewCodec(&buf, &buf)

	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      "1",
		Result:  map[string]interface{}{"tools": []interface{}{}},
	}
	if err := codec.WriteMessage(resp); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg, err := codec.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if msg.Type != TypeResponse {
		t.Errorf("expected response, got type %d", msg.Type)
	}
	if msg.Response.ID.(string) != "1" {
		t.Errorf("expected id 1, got %v", msg.Response.ID)
	}
}

func TestCodecErrorResponse(t *testing.T) {
	var buf bytes.Buffer
	codec := NewCodec(&buf, &buf)

	resp := NewErrorResponse("2", mcp.ErrMethodNotFound, "method not found")
	if err := codec.WriteMessage(resp); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg, err := codec.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if msg.Response.Error.Code != mcp.ErrMethodNotFound {
		t.Errorf("expected error code %d, got %d", mcp.ErrMethodNotFound, msg.Response.Error.Code)
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse("test-id", -32001, "custom error")
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}
	if resp.Error.Code != -32001 {
		t.Errorf("expected code -32001, got %d", resp.Error.Code)
	}
	if resp.ID.(string) != "test-id" {
		t.Errorf("expected id test-id, got %v", resp.ID)
	}
}

func TestCodecWriteRaw(t *testing.T) {
	var buf bytes.Buffer
	codec := NewCodec(&buf, &buf)

	raw := []byte(`{"jsonrpc":"2.0","id":"99","result":{}}` + "\n")
	if err := codec.WriteRaw(raw); err != nil {
		t.Fatalf("write raw: %v", err)
	}

	msg, err := codec.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if msg.Type != TypeResponse {
		t.Errorf("expected response, got %d", msg.Type)
	}
}

func TestProtocolVersion(t *testing.T) {
	resp := NewSuccessResponse("1", mcp.InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: mcp.Implementation{
			Name:    "test-server",
			Version: "1.0.0",
		},
	})

	data, _ := json.Marshal(resp)

	var parsed mcp.JSONRPCResponse
	json.Unmarshal(data, &parsed)

	result, _ := json.Marshal(parsed.Result)
	var initResult mcp.InitializeResult
	json.Unmarshal(result, &initResult)

	if initResult.ProtocolVersion != "2024-11-05" {
		t.Errorf("expected protocol version 2024-11-05, got %s", initResult.ProtocolVersion)
	}
	if initResult.ServerInfo.Name != "test-server" {
		t.Errorf("expected server name test-server, got %s", initResult.ServerInfo.Name)
	}
}
