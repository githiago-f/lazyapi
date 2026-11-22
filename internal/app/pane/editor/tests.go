package editor

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/script"
	"github.com/githiago-f/lazyapi/internal/store"
)

const defaultTestScript = `// JavaScript/TypeScript test script. F5 runs it against the last response.
// Globals: request, response, env, vars, lazyapi, test, expect
// (run "lazyapi types" for the full API). URL imports are allowed:
//   import { z } from "https://esm.sh/zod@3.23.8";

test("status is 2xx", () => {
  expect(response.status).toBeGreaterThanOrEqual(200);
  expect(response.status).toBeLessThan(300);
});

test("matches the OpenAPI schema", () => {
  expect(response).toMatchSchema();
});
`

var testResultStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder(), false, true, false, false).
	BorderForeground(lipgloss.Color(config.Overlay1))

var testPassStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Green))
var testFailStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Red))
var testInfoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Subtext0))

type testsPane struct {
	active   bool
	width    int
	height   int
	script   components.Text
	results  string
	reqData  model.Request
	envStore *env.Store

	hasResponse    bool
	respStatus     int
	respStatusText string
	respHeaders    http.Header
	respBody       string
}

func TestsTab() *testsPane {
	return &testsPane{
		script:  components.NewTextarea("Test script (JavaScript)", defaultTestScript),
		results: "",
	}
}

func (tp *testsPane) SetActive(b bool) {
	tp.active = b
	if b {
		tp.script.Focus()
	} else {
		tp.script.Blur()
	}
}

func (tp *testsPane) IsActive() bool {
	return tp.active
}

func (tp *testsPane) SetEnvStore(s *env.Store) {
	tp.envStore = s
}

func (tp *testsPane) SetRequestData(req model.Request) {
	tp.reqData = req
}

func (tp *testsPane) SetResponseData(statusCode int, statusText string, header http.Header, body string) {
	tp.hasResponse = true
	tp.respStatus = statusCode
	tp.respStatusText = statusText
	tp.respHeaders = header
	tp.respBody = body
}

func (tp *testsPane) SetResponseError() {
	tp.hasResponse = false
	tp.respStatus = 0
	tp.respStatusText = ""
	tp.respHeaders = nil
	tp.respBody = ""
}

func (tp *testsPane) SetValue(script string) {
	if script == "" {
		tp.script.SetValue(defaultTestScript)
	} else {
		tp.script.SetValue(script)
	}
}

// Value returns the script to persist. The untouched default template counts
// as "no tests" so it is never written next to the spec.
func (tp *testsPane) Value() string {
	v := tp.script.Value()
	if strings.TrimSpace(v) == strings.TrimSpace(defaultTestScript) {
		return ""
	}
	return v
}

func (tp *testsPane) SetResults(result script.Result) {
	tp.results = formatTestResults(result)
}

func (tp *testsPane) Reset() {
	tp.results = ""
	tp.script.SetValue(defaultTestScript)
	tp.hasResponse = false
}

func (tp *testsPane) HelpBindings() []key.Binding {
	return []key.Binding{
		config.DefaultKeyMap.Back,
		config.DefaultKeyMap.RunTests,
	}
}

func (tp testsPane) Init() tea.Cmd {
	return nil
}

func (tp testsPane) View() tea.View {
	activeColor := config.DefaultConfig.PrimaryColor()

	editorHeight := max(1, tp.height*7/10)
	resultsHeight := max(3, tp.height-3-editorHeight)

	tp.script.Style = tp.script.Style.
		UnsetBorderForeground().
		Width(max(0, tp.width-2)).
		Height(editorHeight)
	if tp.active {
		tp.script.Style = tp.script.Style.BorderForeground(activeColor)
	}
	tp.script.TextArea.SetWidth(max(0, tp.width-3))
	tp.script.TextArea.SetHeight(editorHeight)

	var results string
	if tp.results == "" {
		results = testInfoStyle.Render("Press F5 to run tests")
	} else {
		results = tp.results
	}

	resultsStyle := testResultStyle.Width(max(0, tp.width-2)).Height(resultsHeight)

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		tp.script.View().Content,
		resultsStyle.Render(results),
	)

	return tea.NewView(content)
}

func (tp *testsPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		tp.width = msg.Width
		tp.height = msg.Height

	case tabs.SetActiveTabMsg:
		tp.SetActive(msg.Active)
		return tp, nil

	case tea.KeyPressMsg:
		if !tp.active {
			return tp, nil
		}

		if key.Matches(msg, config.DefaultKeyMap.RunTests) {
			return tp, tp.runTestsCmd()
		}

		model, cmd := tp.script.Update(msg)
		tp.script, _ = model.(components.Text)
		return tp, cmd
	}

	return tp, nil
}

func (tp testsPane) buildExecData() script.ExecData {
	data := script.ExecData{Request: tp.reqData}

	if tp.envStore != nil {
		envMap, _ := tp.envStore.Load()
		data.Env = envMap
	} else {
		data.Env = tp.reqData.Env
	}

	if tp.hasResponse {
		data.Response = &script.Response{
			Status:     tp.respStatus,
			StatusText: tp.respStatusText,
			Header:     tp.respHeaders,
			Body:       tp.respBody,
			Ref:        tp.reqData.OpenAPIRef,
		}
	}

	return data
}

func (tp testsPane) runTestsCmd() tea.Cmd {
	source := tp.script.Value()
	data := tp.buildExecData()
	req := tp.reqData

	return func() tea.Msg {
		return TestsResultMsg{Result: runScript(source, data, req)}
	}
}

// runScript runs source with the request's spec as context, so
// lazyapi.send(), toMatchSchema() and imports relative to the script work.
func runScript(source string, data script.ExecData, req model.Request) script.Result {
	opts := script.Options{Filename: "tests.js"}
	if req.FileName == "" {
		return script.Run(source, data, opts)
	}

	spec, err := store.ParseSpec(req.FileName)
	if err != nil {
		return script.Result{ScriptError: err.Error()}
	}
	specDir := filepath.Dir(req.FileName)
	opts.ResolveDir = filepath.Join(specDir, store.TestsDir)
	opts.Root = specDir
	if req.OpenAPIRef != nil {
		if op := store.FindOperation(spec, *req.OpenAPIRef); op != nil {
			if o, err := script.OperationOptions(req.FileName, op, *req.OpenAPIRef); err == nil {
				opts = o
			}
		}
	}
	opts.Modules = script.NewModuleLoader(specDir)

	ctx := &script.SpecContext{
		Spec:      spec,
		SpecPath:  req.FileName,
		ServerURL: req.ServerURL,
		Env:       data.Env,
		Vars:      map[string]string{},
	}
	ctx.Bind(&data)

	result := script.Run(source, data, opts)
	if err := opts.Modules.SaveLock(); err != nil && result.ScriptError == "" {
		result.ScriptError = err.Error()
	}
	return result
}

func formatTestResults(result script.Result) string {
	var b strings.Builder

	if result.ScriptError != "" {
		fmt.Fprintf(&b, "Script error:\n%s\n\n", result.ScriptError)
	}

	if len(result.Results) == 0 && result.ScriptError == "" {
		b.WriteString("No tests defined.\n")
		b.WriteString("Use:  test(\"name\", () => expect(response.status).toBe(200))\n")
	}

	for _, tr := range result.Results {
		var mark string
		if tr.Passed {
			mark = testPassStyle.Render("PASS")
		} else {
			mark = testFailStyle.Render("FAIL")
		}
		fmt.Fprintf(&b, "  %s %s\n", mark, tr.Name)
		if tr.Message != "" {
			fmt.Fprintf(&b, "       %s\n", testInfoStyle.Render(tr.Message))
		}
	}

	if len(result.Results) > 0 {
		b.WriteString("\n")
		fmt.Fprintf(&b, "%d passed, %d failed\n", result.PassCount, result.FailCount)
	}

	if len(result.Logs) > 0 {
		b.WriteString("\nConsole:\n")
		for _, l := range result.Logs {
			fmt.Fprintf(&b, "  %s %s\n", testInfoStyle.Render("["+l.Level+"]"), l.Message)
		}
	}

	return b.String()
}
