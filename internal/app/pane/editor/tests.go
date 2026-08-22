package editor

import (
	"fmt"
	"net/http"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/lua"
	"github.com/githiago-f/lazyapi/internal/model"
)

const defaultTestScript = `-- Write Lua test scripts here.
-- Available globals: request, response, env, tests

tests["Status is 200"] = response.status() == 200
tests["Content-Type is JSON"] = response.header("Content-Type"):find("json") ~= nil

-- Helper functions:
-- json_decode(str)    — decode JSON string to table
-- json_encode(table)  — encode table to JSON string
-- test("name", fn)    — run fn in protected mode, record result
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
		script:  components.NewTextarea("Lua test script", defaultTestScript),
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

func (tp *testsPane) Value() string {
	return tp.script.Value()
}

func (tp *testsPane) SetResults(result lua.ScriptResult) {
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

func (tp testsPane) buildExecData() lua.ExecData {
	data := lua.ExecData{
		Request: tp.reqData,
	}

	if tp.envStore != nil {
		envMap, _ := tp.envStore.Load()
		data.Env = envMap
	} else {
		data.Env = tp.reqData.Env
	}

	if tp.hasResponse {
		data.HasResponse = true
		data.RespStatus = tp.respStatus
		data.RespStatusText = tp.respStatusText
		data.RespHeaders = tp.respHeaders
		data.RespBody = tp.respBody
	}

	return data
}

func (tp testsPane) runTestsCmd() tea.Cmd {
	script := tp.script.Value()
	data := tp.buildExecData()

	return func() tea.Msg {
		result := lua.RunScript(script, data)
		return TestsResultMsg{Result: result}
	}
}

func formatTestResults(result lua.ScriptResult) string {
	var b strings.Builder

	if result.ScriptError != "" {
		fmt.Fprintf(&b, "Script error:\n%s\n\n", result.ScriptError)
	}

	if len(result.Results) == 0 && result.ScriptError == "" {
		b.WriteString("No tests defined.\n")
		b.WriteString("Use:  tests[\"description\"] = true/false\n")
		return b.String()
	}

	for _, tr := range result.Results {
		var mark string
		if tr.Passed {
			mark = testPassStyle.Render("PASS")
		} else {
			mark = testFailStyle.Render("FAIL")
		}
		fmt.Fprintf(&b, "  %s %s\n", mark, tr.Name)
	}

	if len(result.Results) > 0 {
		b.WriteString("\n")
		fmt.Fprintf(&b, "%d passed, %d failed\n", result.PassCount, result.FailCount)
	}

	return b.String()
}
