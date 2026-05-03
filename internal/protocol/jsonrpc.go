package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/JinBeiCN/mcp-gateway/pkg/mcp"
)

type MessageType int

const (
	TypeRequest MessageType = iota
	TypeResponse
	TypeNotification
)

type ParsedMessage struct {
	Type    MessageType
	Raw     json.RawMessage
	Request *mcp.JSONRPCRequest
	Response *mcp.JSONRPCResponse
	Notification *mcp.JSONRPCNotification
}

type Codec struct {
	reader  *bufio.Reader
	writer  io.Writer
	mu      sync.Mutex
}

func NewCodec(r io.Reader, w io.Writer) *Codec {
	return &Codec{
		reader: bufio.NewReader(r),
		writer: w,
	}
}

func (c *Codec) ReadMessage() (*ParsedMessage, error) {
	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("read line: %w", err)
	}

	msg := &ParsedMessage{Raw: line}

	// Determine message type by checking for method/result fields
	var probe struct {
		Method string          `json:"method"`
		ID     json.RawMessage `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(line, &probe); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	// Request: has method and id
	if probe.Method != "" {
		req := &mcp.JSONRPCRequest{}
		if err := json.Unmarshal(line, req); err != nil {
			return nil, fmt.Errorf("parse request: %w", err)
		}
		msg.Type = TypeRequest
		msg.Request = req
		return msg, nil
	}

	// Response: has id and (result or error)
	if probe.ID != nil && (probe.Result != nil || probe.Error != nil) {
		resp := &mcp.JSONRPCResponse{}
		if err := json.Unmarshal(line, resp); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		msg.Type = TypeResponse
		msg.Response = resp
		return msg, nil
	}

	// Notification: has method, no id
	if probe.Method != "" && probe.ID == nil {
		notif := &mcp.JSONRPCNotification{}
		if err := json.Unmarshal(line, notif); err != nil {
			return nil, fmt.Errorf("parse notification: %w", err)
		}
		msg.Type = TypeNotification
		msg.Notification = notif
		return msg, nil
	}

	return nil, fmt.Errorf("unrecognized JSON-RPC message")
}

func (c *Codec) WriteMessage(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')
	if _, err := c.writer.Write(data); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

func (c *Codec) WriteRaw(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.writer.Write(data); err != nil {
		return fmt.Errorf("write raw: %w", err)
	}
	return nil
}

func NewErrorResponse(id interface{}, code int, message string) *mcp.JSONRPCResponse {
	return &mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcp.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
}

func NewSuccessResponse(id interface{}, result interface{}) *mcp.JSONRPCResponse {
	return &mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}
