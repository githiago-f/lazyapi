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
)

// body is the Body tab: a text area, or name/value rows when the body is a
// form (application/x-www-form-urlencoded or multipart/form-data). Form
// rows are stored as a JSON object, which the request encodes as fields.
type body struct {
	active bool
	editor components.Text

	form     bool
	fields   []paramField
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
	editor := components.NewTextarea("Json data", "")

	return &body{editor: editor, focusPos: -1}
}

// SetForm switches between the text area and form rows. Call it before
// SetValue.
func (b *body) SetForm(form bool) {
	b.form = form
	b.focusPos = -1
}

func (b *body) SetValue(s string) {
	b.editor.SetValue(s)
	if b.form {
		b.fields = formRows(s)
		b.updateFieldWidths()
	}
}

func (b *body) Value() string {
	if !b.form {
		return b.editor.Value()
	}
	return formJSON(b.fields)
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
	if len(rows) == 0 {
		rows = []paramField{createParam()}
	}
	return rows
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

func (b *body) updateFieldWidths() {
	total := b.width - 2
	nameWidth := total / 2
	valueWidth := total - nameWidth
	for i := range b.fields {
		b.fields[i].name.Style = lipgloss.NewStyle().Width(nameWidth)
		b.fields[i].name.TextInput.SetWidth(max(0, nameWidth-2))
		b.fields[i].value.Style = lipgloss.NewStyle().Width(valueWidth)
		b.fields[i].value.TextInput.SetWidth(max(0, valueWidth-2))
	}
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
	b.editor.Style = b.editor.Style.UnsetBorderForeground()
	if b.active {
		b.editor.Style = b.editor.Style.BorderForeground(config.DefaultConfig.PrimaryColor())
	}
	return tea.NewView(b.editor.View().Content)
}

func (b body) formView() string {
	out := titleStyle.Width(b.width).Render("Form fields")
	active := config.DefaultConfig.PrimaryColor()
	for i := range b.fields {
		b.fields[i].name.Style = b.fields[i].name.Style.UnsetBorderForeground()
		b.fields[i].value.Style = b.fields[i].value.Style.UnsetBorderForeground()
	}
	if b.active && b.focusPos >= 0 && b.focusPos/2 < len(b.fields) {
		row, col := b.focusPos/2, b.focusPos%2
		if col == 0 {
			b.fields[row].name.Style = b.fields[row].name.Style.BorderForeground(active)
		} else {
			b.fields[row].value.Style = b.fields[row].value.Style.BorderForeground(active)
		}
	}
	for i := range b.fields {
		out = lipgloss.JoinVertical(lipgloss.Top, out, b.fields[i].View().Content)
	}
	return out
}

func (b body) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		bodyHeight := max(1, msg.Height-5)
		b.editor.Style = b.editor.Style.
			Width(msg.Width - 1).
			Height(bodyHeight)
		b.editor.TextArea.SetWidth(max(0, msg.Width-3))
		b.editor.TextArea.SetHeight(bodyHeight)
		b.width = msg.Width
		b.updateFieldWidths()

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
	model, cmd := b.editor.Update(msg)
	b.editor, _ = model.(components.Text)
	return &b, cmd
}

func (b body) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return &b, nil
	}
	total := len(b.fields) * 2
	switch {
	case key.Matches(k, config.DefaultKeyMap.AddHeader):
		b.fields = append(b.fields, createParam())
		b.updateFieldWidths()
		b.focusPos = len(b.fields)*2 - 2
		return &b, nil
	case key.Matches(k, config.DefaultKeyMap.Next):
		if total > 0 {
			b.focusPos = (b.focusPos + 1) % total
		}
		return &b, nil
	case key.Matches(k, config.DefaultKeyMap.Prev):
		if total > 0 {
			b.focusPos = (b.focusPos - 1 + total) % total
		}
		return &b, nil
	}
	if total == 0 || b.focusPos < 0 {
		return &b, nil
	}
	row, col := b.focusPos/2, b.focusPos%2
	if col == 0 {
		m, cmd := b.fields[row].name.Update(k)
		b.fields[row].name = m.(components.Field)
		return &b, cmd
	}
	m, cmd := b.fields[row].value.Update(k)
	b.fields[row].value = m.(components.Field)
	return &b, cmd
}
