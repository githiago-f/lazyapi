// Package lua provides embedded Lua scripting for test execution.
//
// The engine exposes three global tables to Lua scripts:
//
//   - request  — details of the outgoing HTTP request
//   - response — details of the received HTTP response (nil until a request is sent)
//   - env      — environment variables and test-scope variables
//
// Additionally a global `tests` table is provided; scripts set
// `tests["description"] = true/false` to report pass/fail results.
package lua

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/githiago-f/lazyapi/internal/model"
	lua "github.com/yuin/gopher-lua"
)

// TestResult is a single assertion result from a Lua script.
type TestResult struct {
	Name    string
	Passed  bool
	Message string // failure reason, when known (e.g. the error raised inside test())
}

// ScriptResult holds the outcome of running a Lua test script.
type ScriptResult struct {
	Results     []TestResult
	ScriptError string // non-empty if the script panicked or had a syntax error
	PassCount   int
	FailCount   int
	Output      string // pre-formatted, human-readable summary
}

// ExecData bundles the request/response/environment context that the
// Lua engine exposes to scripts.
type ExecData struct {
	Request model.Request
	Env     map[string]string

	// Response fields — HasResponse == false means no response yet
	HasResponse    bool
	RespStatus     int
	RespStatusText string
	RespHeaders    http.Header
	RespBody       string
}

// RunScript executes a Lua test script and returns the collected results.
func RunScript(script string, data ExecData) ScriptResult {
	L := lua.NewState()
	defer L.Close()

	L.OpenLibs()

	rec := &recorder{}
	L.SetGlobal("tests", newTestsTable(L, rec))

	registerRequest(L, data.Request)
	registerEnv(L, data.Env)

	if data.HasResponse {
		registerResponse(L, data.RespStatus, data.RespStatusText, data.RespHeaders, data.RespBody)
	} else {
		registerNilResponse(L)
	}

	registerHelpers(L, rec)

	var scriptErr string
	if err := L.DoString(script); err != nil {
		scriptErr = err.Error()
	}

	result := collectResults(rec, scriptErr)
	result.Output = formatOutput(result, scriptErr)
	return result
}

// ---------------------------------------------------------------------------
// Request API
// ---------------------------------------------------------------------------

func registerRequest(L *lua.LState, req model.Request) {
	tbl := L.NewTable()

	L.SetField(tbl, "method", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(req.Method.Label()))
		return 1
	}))

	L.SetField(tbl, "uri", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(req.URI))
		return 1
	}))

	L.SetField(tbl, "url", L.NewFunction(func(L *lua.LState) int {
		server := req.ServerURL
		if server != "" {
			server = strings.TrimRight(server, "/") + req.URI
		} else {
			server = req.URI
		}
		L.Push(lua.LString(server))
		return 1
	}))

	L.SetField(tbl, "body", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(req.Body.Raw))
		return 1
	}))

	L.SetField(tbl, "header", L.NewFunction(func(L *lua.LState) int {
		name := L.ToString(1)
		L.Push(luaHeaderLookup(req.Headers, name))
		return 1
	}))

	L.SetField(tbl, "headers", L.NewFunction(func(L *lua.LState) int {
		L.Push(luaStringMap(L, req.Headers))
		return 1
	}))

	L.SetField(tbl, "query", L.NewFunction(func(L *lua.LState) int {
		name := L.ToString(1)
		L.Push(luaHeaderLookup(req.Query, name))
		return 1
	}))

	L.SetField(tbl, "param", L.NewFunction(func(L *lua.LState) int {
		name := L.ToString(1)
		L.Push(luaHeaderLookup(req.Params, name))
		return 1
	}))

	L.SetGlobal("request", tbl)
}

// ---------------------------------------------------------------------------
// Response API
// ---------------------------------------------------------------------------

func registerResponse(L *lua.LState, status int, statusText string, headers http.Header, body string) {
	tbl := L.NewTable()

	L.SetField(tbl, "status", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(status))
		return 1
	}))

	L.SetField(tbl, "statusText", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(statusText))
		return 1
	}))

	L.SetField(tbl, "body", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(body))
		return 1
	}))

	L.SetField(tbl, "header", L.NewFunction(func(L *lua.LState) int {
		name := L.ToString(1)
		val := headers.Get(name)
		if val == "" {
			L.Push(lua.LNil)
		} else {
			L.Push(lua.LString(val))
		}
		return 1
	}))

	L.SetField(tbl, "headers", L.NewFunction(func(L *lua.LState) int {
		t := L.NewTable()
		for k, vs := range headers {
			if len(vs) > 0 {
				L.SetField(t, k, lua.LString(vs[0]))
			}
		}
		L.Push(t)
		return 1
	}))

	L.SetField(tbl, "json", L.NewFunction(func(L *lua.LState) int {
		val, err := jsonDecode(L, body)
		if err != nil {
			L.RaiseError("response.json(): %s", err.Error())
			return 0
		}
		L.Push(val)
		return 1
	}))

	L.SetGlobal("response", tbl)
}

func registerNilResponse(L *lua.LState) {
	tbl := L.NewTable()

	L.SetField(tbl, "status", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(0))
		return 1
	}))

	L.SetField(tbl, "statusText", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(""))
		return 1
	}))

	L.SetField(tbl, "body", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(""))
		return 1
	}))

	L.SetField(tbl, "header", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNil)
		return 1
	}))

	L.SetField(tbl, "headers", L.NewFunction(func(L *lua.LState) int {
		L.Push(L.NewTable())
		return 1
	}))

	L.SetField(tbl, "json", L.NewFunction(func(L *lua.LState) int {
		L.RaiseError("response.json(): no response available — send a request first")
		return 0
	}))

	L.SetGlobal("response", tbl)
}

// ---------------------------------------------------------------------------
// Environment API
// ---------------------------------------------------------------------------

func registerEnv(L *lua.LState, envMap map[string]string) {
	tbl := L.NewTable()

	L.SetField(tbl, "get", L.NewFunction(func(L *lua.LState) int {
		name := L.ToString(1)
		val := envMap[name]
		if val == "" {
			L.Push(lua.LNil)
		} else {
			L.Push(lua.LString(val))
		}
		return 1
	}))

	L.SetField(tbl, "has", L.NewFunction(func(L *lua.LState) int {
		name := L.ToString(1)
		L.Push(lua.LBool(envMap[name] != ""))
		return 1
	}))

	L.SetField(tbl, "set", L.NewFunction(func(L *lua.LState) int {
		name := L.ToString(1)
		val := L.ToString(2)
		envMap[name] = val
		return 0
	}))

	L.SetField(tbl, "vars", L.NewFunction(func(L *lua.LState) int {
		L.Push(luaStringMap(L, envMap))
		return 1
	}))

	L.SetGlobal("env", tbl)
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func registerHelpers(L *lua.LState, rec *recorder) {
	// json_decode(string) -> table
	L.SetGlobal("json_decode", L.NewFunction(func(L *lua.LState) int {
		str := L.ToString(1)
		val, err := jsonDecode(L, str)
		if err != nil {
			L.RaiseError("json_decode: %s", err.Error())
			return 0
		}
		L.Push(val)
		return 1
	}))

	// json_encode(table) -> string
	L.SetGlobal("json_encode", L.NewFunction(func(L *lua.LState) int {
		str, err := jsonEncode(L.Get(1))
		if err != nil {
			L.RaiseError("json_encode: %s", err.Error())
			return 0
		}
		L.Push(lua.LString(str))
		return 1
	}))

	// test(description, fn) — runs fn in protected mode, records result
	L.SetGlobal("test", L.NewFunction(func(L *lua.LState) int {
		desc := L.ToString(1)
		fn := L.CheckFunction(2)

		L.Push(fn)
		if err := L.PCall(0, 0, nil); err != nil {
			rec.add(desc, false, errorMessage(err))
		} else {
			rec.add(desc, true, "")
		}
		return 0
	}))
}

// ---------------------------------------------------------------------------
// Test recording
// ---------------------------------------------------------------------------

// recorder keeps assertions in the order the script produced them. Repeated
// names are kept as separate results instead of overwriting each other.
type recorder struct {
	results []TestResult
}

func (r *recorder) add(name string, passed bool, message string) {
	r.results = append(r.results, TestResult{Name: name, Passed: passed, Message: message})
}

// newTestsTable returns a proxy table: every `tests[name] = value` assignment
// is recorded in order, and reads return the last value assigned to a name.
func newTestsTable(L *lua.LState, rec *recorder) *lua.LTable {
	shadow := L.NewTable()
	proxy := L.NewTable()
	meta := L.NewTable()
	L.SetField(meta, "__newindex", L.NewFunction(func(L *lua.LState) int {
		key := L.Get(2)
		val := L.Get(3)
		passed := luaToBool(val)
		msg := ""
		if !passed {
			msg = fmt.Sprintf("expected truthy value, got %s", val.String())
		}
		rec.add(key.String(), passed, msg)
		shadow.RawSet(key, val)
		return 0
	}))
	L.SetField(meta, "__index", shadow)
	L.SetMetatable(proxy, meta)
	return proxy
}

// errorMessage strips the Lua stack traceback from a PCall error so the
// report only carries the assertion message.
func errorMessage(err error) string {
	if apiErr, ok := err.(*lua.ApiError); ok {
		msg := apiErr.Object.String()
		if msg != "" {
			return msg
		}
	}
	msg := err.Error()
	if idx := strings.Index(msg, "\nstack traceback:"); idx >= 0 {
		msg = msg[:idx]
	}
	return msg
}

// ---------------------------------------------------------------------------
// Result collection & formatting
// ---------------------------------------------------------------------------

func collectResults(rec *recorder, scriptErr string) ScriptResult {
	result := ScriptResult{
		Results:     append([]TestResult{}, rec.results...),
		ScriptError: scriptErr,
	}
	for _, tr := range result.Results {
		if tr.Passed {
			result.PassCount++
		} else {
			result.FailCount++
		}
	}
	return result
}

func formatOutput(result ScriptResult, scriptErr string) string {
	var b strings.Builder

	if scriptErr != "" {
		fmt.Fprintf(&b, "Script error: %s\n\n", scriptErr)
	}

	if len(result.Results) == 0 && scriptErr == "" {
		b.WriteString("No tests defined.\n")
		b.WriteString("Use:  tests[\"description\"] = true/false\n")
		b.WriteString("or:   test(\"description\", function() assert(...) end)\n")
		return b.String()
	}

	for _, tr := range result.Results {
		mark := "PASS"
		if !tr.Passed {
			mark = "FAIL"
		}
		fmt.Fprintf(&b, "  [%s] %s\n", mark, tr.Name)
		if tr.Message != "" {
			fmt.Fprintf(&b, "         %s\n", tr.Message)
		}
	}

	if len(result.Results) > 0 {
		b.WriteString("\n")
		fmt.Fprintf(&b, "%d passed, %d failed\n", result.PassCount, result.FailCount)
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// JSON helpers
// ---------------------------------------------------------------------------

func jsonDecode(L *lua.LState, str string) (lua.LValue, error) {
	var val any
	if err := json.Unmarshal([]byte(str), &val); err != nil {
		return lua.LNil, err
	}
	return goValueToLua(L, val), nil
}

func jsonEncode(lv lua.LValue) (string, error) {
	val := luaValueToGo(lv)
	data, err := json.Marshal(val)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func goValueToLua(L *lua.LState, val any) lua.LValue {
	switch v := val.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []any:
		tbl := L.NewTable()
		for i, item := range v {
			tbl.RawSetInt(i+1, goValueToLua(L, item))
		}
		return tbl
	case map[string]any:
		tbl := L.NewTable()
		for k, item := range v {
			tbl.RawSetString(k, goValueToLua(L, item))
		}
		return tbl
	default:
		return lua.LString(fmt.Sprintf("%v", v))
	}
}

func luaValueToGo(lv lua.LValue) any {
	switch v := lv.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return float64(v)
	case lua.LString:
		return v.String()
	case *lua.LTable:
		maxSeq := 0
		seqMap := make(map[int]any)
		hashMap := make(map[string]any)
		isArray := true

		v.ForEach(func(k, val lua.LValue) {
			if kn, ok := k.(lua.LNumber); ok {
				idx := int(kn)
				if idx > 0 && idx <= 1000000 {
					seqMap[idx] = luaValueToGo(val)
					if idx > maxSeq {
						maxSeq = idx
					}
					return
				}
			}
			isArray = false
			hashMap[k.String()] = luaValueToGo(val)
		})

		if isArray && maxSeq > 0 {
			arr := make([]any, maxSeq)
			for i := 1; i <= maxSeq; i++ {
				if val, ok := seqMap[i]; ok {
					arr[i-1] = val
				} else {
					arr[i-1] = nil
				}
			}
			return arr
		}
		return hashMap
	default:
		return lv.String()
	}
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

func luaToBool(lv lua.LValue) bool {
	switch v := lv.(type) {
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return int(v) != 0
	case lua.LString:
		s := v.String()
		return s != "" && s != "false" && s != "0"
	case *lua.LNilType:
		return false
	default:
		return false
	}
}

func luaHeaderLookup(headers map[string]string, name string) lua.LValue {
	val := headers[name]
	if val == "" {
		for k, v := range headers {
			if strings.EqualFold(k, name) {
				val = v
				break
			}
		}
	}
	if val == "" {
		return lua.LNil
	}
	return lua.LString(val)
}

func luaStringMap(L *lua.LState, m map[string]string) *lua.LTable {
	tbl := L.NewTable()
	for k, v := range m {
		L.SetField(tbl, k, lua.LString(v))
	}
	return tbl
}
