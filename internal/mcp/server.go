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
	"sync/atomic"

	"github.com/githiago-f/lazyapi/internal/permit"
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
before(req) to change the request (e.g. add auth) before it is sent.
Requests only reach servers the user allowed: the user is asked first when
the client can show the question. A result saying "lazyapi blocked" means the
user has not allowed (or refused) that server: tell the user, and never run
"lazyapi allow" or change the MCP server's flags yourself.`

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	// Result and Error are set on the client's answers to our requests.
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

// outgoing is a request the server sends to the client (elicitation).
type outgoing struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type reply struct {
	result json.RawMessage
	err    *rpcError
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

	// canElicit is set when the client declared the elicitation capability
	// at initialize: the server may then ask the user (permissions).
	canElicit atomic.Bool
	nextID    atomic.Int64
	pmu       sync.Mutex
	pending   map[string]chan reply
}

// Serve reads requests from in and writes responses to out until in closes
// or ctx is cancelled. Tool calls run one at a time, off the reading loop,
// so the client's answers to the server's own requests (asking the user to
// allow a server) are read while a call waits for them.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	s.out = out
	calls := make(chan request, 64)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for req := range calls {
			s.answer(ctx, req)
		}
	}()
	defer func() {
		close(calls)
		s.failPending("the client closed the connection")
		wg.Wait()
	}()

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
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			s.write(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: errParse, Message: err.Error()}})
			continue
		}
		switch {
		case req.Method == "" && len(req.ID) > 0:
			s.deliver(req)
		case req.Method == "tools/call" && len(req.ID) > 0:
			calls <- req
		default:
			s.answer(ctx, req)
		}
	}
	return scanner.Err()
}

func (s *Server) answer(ctx context.Context, req request) {
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
			Capabilities    struct {
				Elicitation json.RawMessage `json:"elicitation"`
			} `json:"capabilities"`
		}
		_ = json.Unmarshal(req.Params, &p)
		s.canElicit.Store(len(p.Capabilities.Elicitation) > 0 && string(p.Capabilities.Elicitation) != "null")
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

// deliver hands the client's answer to the request waiting for it.
func (s *Server) deliver(r request) {
	var id string
	if json.Unmarshal(r.ID, &id) != nil {
		return
	}
	s.pmu.Lock()
	ch := s.pending[id]
	delete(s.pending, id)
	s.pmu.Unlock()
	if ch != nil {
		ch <- reply{result: r.Result, err: r.Error}
	}
}

func (s *Server) failPending(msg string) {
	s.pmu.Lock()
	defer s.pmu.Unlock()
	for id, ch := range s.pending {
		ch <- reply{err: &rpcError{Code: -32603, Message: msg}}
		delete(s.pending, id)
	}
}

// call sends a request to the client and waits for its answer.
func (s *Server) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := fmt.Sprintf("lazyapi-%d", s.nextID.Add(1))
	ch := make(chan reply, 1)
	s.pmu.Lock()
	if s.pending == nil {
		s.pending = map[string]chan reply{}
	}
	s.pending[id] = ch
	s.pmu.Unlock()

	data, err := json.Marshal(outgoing{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	_, err = s.out.Write(append(data, '\n'))
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	select {
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf("%s: %s", method, r.err.Message)
		}
		return r.result, nil
	case <-ctx.Done():
		s.pmu.Lock()
		delete(s.pending, id)
		s.pmu.Unlock()
		return nil, ctx.Err()
	}
}

// Asker asks the user through the client (MCP elicitation) whether a
// request may go to a server; nil when the client cannot show the question.
func (s *Server) Asker() permit.Asker {
	if !s.canElicit.Load() {
		return nil
	}
	return s.askPermission
}

func (s *Server) askPermission(ctx context.Context, p permit.Prompt) (permit.Scope, error) {
	var (
		message string
		options []string
		names   []string
	)
	switch {
	case p.Loopback:
		message = fmt.Sprintf("lazyapi (%s) wants to send %s %s.\n\nAllow requests to servers on this machine (localhost, any port)? You are asked only once.", p.Tool, p.Method, p.URL)
		options = []string{string(permit.Always), string(permit.Deny)}
		names = []string{"Allow local servers", "Deny"}
	default:
		what := "read from (GET, HEAD, OPTIONS)"
		if p.Write {
			what = "change data on (" + p.Method + ", POST, PUT, PATCH, DELETE)"
		}
		message = fmt.Sprintf("lazyapi (%s) wants to send %s %s.\n\nAllow it to %s %s?", p.Tool, p.Method, p.URL, what, p.Origin)
		options = []string{string(permit.Call), string(permit.Session), string(permit.Always), string(permit.Deny)}
		names = []string{"Only for this request", "For this session", "Always (remembered)", "Deny"}
	}
	params := map[string]any{
		"message": message,
		"requestedSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"allow": map[string]any{
					"type": "string", "title": "Allow?", "enum": options, "enumNames": names, "default": string(permit.Deny),
				},
			},
			"required": []string{"allow"},
		},
	}
	raw, err := s.call(ctx, "elicitation/create", params)
	if err != nil {
		return permit.Deny, err
	}
	var r struct {
		Action  string `json:"action"`
		Content struct {
			Allow string `json:"allow"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return permit.Deny, err
	}
	if r.Action != "accept" {
		return permit.Deny, nil // declined or dismissed
	}
	switch scope := permit.Scope(r.Content.Allow); scope {
	case permit.Call, permit.Session, permit.Always:
		if p.Loopback || slices.Contains(options, string(scope)) {
			return scope, nil
		}
	}
	return permit.Deny, nil // anything unexpected is a no
}
