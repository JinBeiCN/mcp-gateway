// Mock MCP server for testing - implements a simple echo/tools MCP server over stdio.
// +build ignore

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type CallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type CallResult struct {
	Content []Content `json:"content"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			break
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		var resp Response
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		switch req.Method {
		case "initialize":
			resp.Result = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    "mock-server",
					"version": "1.0.0",
				},
			}

		case "tools/list":
			resp.Result = map[string]interface{}{
				"tools": []Tool{
					{Name: "echo", Description: "Echo back the input", InputSchema: InputSchema{
						Type: "object",
						Properties: map[string]Property{
							"message": {Type: "string", Description: "The message to echo"},
						},
					}},
					{Name: "add", Description: "Add two numbers", InputSchema: InputSchema{
						Type: "object",
						Properties: map[string]Property{
							"a": {Type: "number", Description: "First number"},
							"b": {Type: "number", Description: "Second number"},
						},
					}},
					{Name: "dangerous_tool", Description: "A tool that should be restricted", InputSchema: InputSchema{
						Type:       "object",
						Properties: map[string]Property{},
					}},
				},
			}

		case "tools/call":
			var params CallParams
			data, _ := json.Marshal(req.Params)
			json.Unmarshal(data, &params)

			switch params.Name {
			case "echo":
				msg := "echoed"
				if m, ok := params.Arguments["message"].(string); ok {
					msg = m
				}
				resp.Result = CallResult{
					Content: []Content{{Type: "text", Text: fmt.Sprintf("Echo: %s", msg)}},
				}
			case "add":
				a, _ := params.Arguments["a"].(float64)
				b, _ := params.Arguments["b"].(float64)
				resp.Result = CallResult{
					Content: []Content{{Type: "text", Text: fmt.Sprintf("Result: %.0f", a+b)}},
				}
			default:
				resp.Result = CallResult{
					Content: []Content{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", params.Name)}},
				}
			}

		case "ping":
			resp.Result = map[string]interface{}{}

		default:
			resp.Error = &Error{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}
		}

		data, _ := json.Marshal(resp)
		fmt.Fprintf(writer, "%s\n", data)
	}
}
