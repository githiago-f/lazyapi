// Package mcp serves LazyAPI's agent tools over the Model Context Protocol
// (JSON-RPC 2.0, newline-delimited, on stdio).
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
)

// ProtocolVersions are the MCP revisions this server speaks, newest first.
var ProtocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

const instructions = `LazyAPI tests HTTP APIs described by OpenAPI specs.
Typical loop: list_specs → list_operations → describe_operation → send_request
(inspect status, body and failed checks) → write_test (a JavaScript test script
saved next to the spec) → run_tests. Every response is validated against the
OpenAPI schema automatically. Call script_api for the test script reference.
run_tests only runs GET/HEAD/OPTIONS unless "methods" says otherwise.
For scenarios across operations (login → create → read → delete) write a flow
with write_flow and run it with run_flow. A test script may export
before(req) to change the request (e.g. add auth) before it is sent.`

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	errParse          = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
)

// Server is an MCP server exposing Tools.
type Server struct {
	Name    string
	Version string
	Tools   *Tools

	mu  sync.Mutex
	out io.Writer
}

// Serve reads requests from in and writes responses to out until in closes
// or ctx is cancelled. Requests are handled one at a time.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	s.out = out
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		s.handleLine(ctx, line)
	}
	return scanner.Err()
}

func (s *Server) handleLine(ctx context.Context, line []byte) {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		s.write(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: errParse, Message: err.Error()}})
		return
	}
	isNotification := len(req.ID) == 0
	result, rerr := s.dispatch(ctx, req)
	if isNotification {
		return
	}
	resp := response{JSONRPC: "2.0", ID: req.ID}
	if rerr != nil {
		resp.Error = rerr
	} else {
		resp.Result = result
	}
	s.write(resp)
}

func (s *Server) dispatch(ctx context.Context, req request) (any, *rpcError) {
	if req.JSONRPC != "2.0" {
		return nil, &rpcError{Code: errInvalidRequest, Message: `jsonrpc must be "2.0"`}
	}
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		version := ProtocolVersions[0]
		if slices.Contains(ProtocolVersions, p.ProtocolVersion) {
			version = p.ProtocolVersion
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
			"instructions":    instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": s.Tools.Definitions()}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{Code: errInvalidParams, Message: err.Error()}
		}
		if !s.Tools.Has(p.Name) {
			return nil, &rpcError{Code: errInvalidParams, Message: fmt.Sprintf("unknown tool %q", p.Name)}
		}
		return toolResult(s.Tools.Call(ctx, p.Name, p.Arguments)), nil
	default:
		if strings.HasPrefix(req.Method, "notifications/") {
			return nil, nil
		}
		return nil, &rpcError{Code: errMethodNotFound, Message: "method not found: " + req.Method}
	}
}

// toolResult wraps a tool's output (or error) as MCP content. Tool errors
// are reported in-band (isError) so the model can read and react to them.
func toolResult(v any, err error) map[string]any {
	if err != nil {
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		}
	}
	var text string
	if s, ok := v.(string); ok {
		text = s
	} else {
		data, mErr := json.MarshalIndent(v, "", "  ")
		if mErr != nil {
			return toolResult(nil, mErr)
		}
		text = string(data)
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": false}
}

func (s *Server) write(resp response) {
	data, err := json.Marshal(resp)
	if err != nil {
		data, _ = json.Marshal(response{JSONRPC: "2.0", ID: resp.ID, Error: &rpcError{Code: -32603, Message: err.Error()}})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.out.Write(append(data, '\n'))
}
