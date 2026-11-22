package editor

import (
	"encoding/json"
	"net/url"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
)

// body is the Body tab: a text area, or name/value rows when the body is a
// form (application/x-www-form-urlencoded or multipart/form-data). Form
// rows are stored as a JSON object, which the request encodes as fields.
type body struct {
	active bool
	editor components.CodeEditor

	form     bool
	grid     kvGrid
	focusPos int
	width    int
}

func (b *body) SetActive(active bool) {
	b.active = active
	if b.form {
		if active {
			b.focusPos = 0
		} else {
			b.focusPos = -1
		}
		return
	}
	if !active {
		b.editor.Blur()
	} else {
		b.editor.Focus()
	}
}

func (b *body) IsActive() bool {
	return b.active
}

func BodyTab() *body {
	editor := components.NewCodeEditor(components.LangJSON, `{ "name": "{{env.USER}}" }  ·  $ or {{ completes variables`, "")

	return &body{editor: editor, focusPos: -1, grid: newGrid()}
}

// SetForm switches between the text area and form rows. Call it before
// SetValue.
func (b *body) SetForm(form bool) {
	b.form = form
	b.focusPos = -1
}

// SetType sets the body's media type: form rows for forms, JSON colors and
// validation for JSON, plain text otherwise. Call it before SetValue.
func (b *body) SetType(t model.MimeType) {
	b.SetForm(model.Body{Type: t}.IsForm())
	switch {
	case t == "" || strings.Contains(string(t), "json"):
		b.editor.Lang = components.LangJSON
	case strings.Contains(string(t), "javascript"):
		b.editor.Lang = components.LangJS
	default:
		b.editor.Lang = components.LangText
	}
}

func (b *body) SetValue(s string) {
	b.editor.SetValue(s)
	if b.form {
		b.grid.setRows(formRows(s))
	}
}

func (b *body) Value() string {
	if !b.form {
		return b.editor.Value()
	}
	return formJSON(b.grid.rows)
}

// formRows reads a JSON object (or an a=1&b=2 string) into rows. Arrays
// become repeated rows; nested values stay JSON.
func formRows(raw string) []paramField {
	var rows []paramField
	add := func(k, v string) { rows = append(rows, createParamWith(k, v)) }
	raw = strings.TrimSpace(raw)
	var obj map[string]any
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	switch {
	case raw == "":
	case dec.Decode(&obj) == nil:
		for _, k := range orderedKeys(raw, obj) {
			switch v := obj[k].(type) {
			case []any:
				for _, item := range v {
					add(k, scalarText(item))
				}
			default:
				add(k, scalarText(v))
			}
		}
	default:
		if vals, err := url.ParseQuery(raw); err == nil {
			for _, k := range sortedKeys(vals) {
				for _, v := range vals[k] {
					add(k, v)
				}
			}
		}
	}
	return rows // setRows adds a blank row when empty
}

// orderedKeys returns obj's keys in the order they appear in raw.
func orderedKeys(raw string, obj map[string]any) []string {
	dec := json.NewDecoder(strings.NewReader(raw))
	var keys []string
	if _, err := dec.Token(); err == nil { // {
		for dec.More() {
			tok, err := dec.Token()
			if err != nil {
				break
			}
			if k, ok := tok.(string); ok {
				keys = append(keys, k)
			}
			var skip json.RawMessage
			if dec.Decode(&skip) != nil {
				break
			}
		}
	}
	if len(keys) != len(obj) {
		keys = keys[:0]
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}
	return keys
}

func sortedKeys(v url.Values) []string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func scalarText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		data, _ := json.Marshal(t)
		return string(data)
	}
}

// formJSON writes the rows as a JSON object in row order; repeated names
// become arrays. Rows without a name are dropped.
func formJSON(rows []paramField) string {
	var order []string
	values := map[string][]string{}
	for _, r := range rows {
		k := strings.TrimSpace(r.name.Value())
		if k == "" {
			continue
		}
		if _, seen := values[k]; !seen {
			order = append(order, k)
		}
		values[k] = append(values[k], r.value.Value())
	}
	if len(order) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("{\n")
	for i, k := range order {
		key, _ := json.Marshal(k)
		b.WriteString("  " + string(key) + ": ")
		vs := values[k]
		if len(vs) == 1 {
			v, _ := json.Marshal(vs[0])
			b.Write(v)
		} else {
			v, _ := json.Marshal(vs)
			b.Write(v)
		}
		if i < len(order)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}")
	return b.String()
}

func (b body) HelpBindings() []key.Binding {
	if b.form {
		return []key.Binding{
			config.DefaultKeyMap.Next,
			config.DefaultKeyMap.Prev,
			key.NewBinding(key.WithKeys(config.DefaultKeyMap.AddHeader.Keys()...), key.WithHelp(config.DefaultKeyMap.AddHeader.Help().Key, "add field")),
			config.DefaultKeyMap.Back,
		}
	}
	return []key.Binding{
		config.DefaultKeyMap.Back,
	}
}

func (b body) Init() tea.Cmd {
	return nil
}

func (b body) View() tea.View {
	if b.form {
		return tea.NewView(b.formView())
	}
	return tea.NewView(b.editor.View().Content)
}

func (b body) formView() string {
	focus := -1
	if b.active {
		focus = b.focusPos
	}
	return lipgloss.JoinVertical(lipgloss.Top, titleStyle.Width(b.width).Render("Form fields"), b.grid.view(focus))
}

func (b body) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.editor.SetSize(msg.Width, max(4, msg.Height-3))
		b.width = msg.Width
		b.grid.setWidth(msg.Width)

	case tabs.SetActiveTabMsg:
		b.SetActive(msg.Active)
		return &b, nil
	}

	if !b.active {
		return &b, nil
	}
	if b.form {
		return b.updateForm(msg)
	}
	var cmd tea.Cmd
	b.editor, cmd = b.editor.Update(msg)
	return &b, cmd
}

func (b body) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return &b, nil
	}
	if b.grid.completing(b.focusPos) && menuKey(k) {
		return &b, b.grid.update(b.focusPos, k)
	}
	total := b.grid.cells()
	switch {
	case key.Matches(k, config.DefaultKeyMap.AddHeader):
		b.focusPos = b.grid.add()
	case key.Matches(k, config.DefaultKeyMap.Next):
		b.focusPos = (b.focusPos + 1) % total
	case key.Matches(k, config.DefaultKeyMap.Prev):
		b.focusPos = (b.focusPos - 1 + total) % total
	default:
		return &b, b.grid.update(b.focusPos, k)
	}
	return &b, nil
}

// Completing reports whether a completion menu is open.
func (b body) Completing() bool {
	if b.form {
		return b.active && b.grid.completing(b.focusPos)
	}
	return b.active && b.editor.Completing()
}
