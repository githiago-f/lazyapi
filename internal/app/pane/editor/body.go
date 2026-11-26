package editor

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"slices"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/app/pane/responses"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

// bodyMode is how the Body tab edits a media type.
type bodyMode int

const (
	modeNone   bodyMode = iota // no body
	modeRaw                    // a text editor (JSON, XML, text, …)
	modeForm                   // name/value rows (urlencoded, multipart)
	modeBinary                 // a local file is the whole body
)

// modeOf picks the editor for a media type: forms get rows, text types the
// editor, anything else (images, PDFs, octet-stream) a file.
func modeOf(t model.MimeType) bodyMode {
	s := strings.ToLower(string(t))
	switch {
	case s == "":
		return modeNone
	case model.Body{Type: t}.IsForm():
		return modeForm
	case strings.HasPrefix(s, "text/"), strings.Contains(s, "json"), strings.Contains(s, "xml"),
		strings.Contains(s, "yaml"), strings.Contains(s, "javascript"), strings.Contains(s, "graphql"):
		return modeRaw
	}
	return modeBinary
}

// bodyTypes are offered in the type select after the spec's own.
var bodyTypes = []string{
	"",
	string(model.ApplicationJSON),
	"application/xml",
	string(model.PlainText),
	string(model.FormURLEncoded),
	string(model.MultipartForm),
	string(model.OctetStream),
}

// Focus stops of the Body tab: the type select, then the content.
const (
	focusType = iota
	focusContent
)

// body is the Body tab: a type select over the content, which is a text
// editor, name/value rows for forms (application/x-www-form-urlencoded,
// multipart/form-data, whose rows can be files), a file path for binary
// types, or nothing. Form rows are stored as a JSON object, which the
// request encodes as fields; file rows go to Body.Files, the binary file
// to Body.File.
type body struct {
	active bool
	focus  int

	kind    components.Selector
	types   []string // the select's types, "" = no body
	applied model.MimeType
	mode    bodyMode

	editor components.CodeEditor

	grid     kvGrid
	focusPos int

	file components.Field

	width, height int
}

func BodyTab() *body {
	editor := components.NewCodeEditor(components.LangJSON, `{ "name": "{{env.USER}}" }  ·  $ or {{ completes variables`, "")
	file := components.InitField(config.T("editor.body.file_placeholder"), "")
	b := &body{
		editor:   editor,
		focusPos: -1,
		grid:     newGrid(),
		file:     file,
		kind:     components.Selector{ID: "editor-body-type", Width: 40},
		focus:    focusContent,
	}
	b.SetTypes(nil)
	b.SetType(model.ApplicationJSON)
	return b
}

// SetTypes sets the types the spec declares for the operation; they come
// first in the select.
func (b *body) SetTypes(declared []string) {
	current := b.applied
	b.types = nil
	for _, t := range append(append([]string{""}, declared...), bodyTypes...) {
		if !slices.Contains(b.types, t) {
			b.types = append(b.types, t)
		}
	}
	b.syncLabels(current)
}

// syncLabels rebuilds the select's labels and points it at t (added to the
// list when it is a type of its own, like application/vnd.api+json).
func (b *body) syncLabels(t model.MimeType) {
	if !slices.Contains(b.types, string(t)) {
		b.types = append(b.types, string(t))
	}
	b.kind.Labels = make([]string, len(b.types))
	for i, ty := range b.types {
		b.kind.Labels[i] = typeLabel(ty)
		if ty == string(t) {
			b.kind.Cursor = i
		}
	}
}

func typeLabel(t string) string {
	switch {
	case t == "":
		return config.T("editor.body.none")
	case modeOf(model.MimeType(t)) == modeBinary:
		return t + " " + config.T("editor.body.file_suffix")
	}
	return t
}

// Type is the body's media type ("" for no body).
func (b *body) Type() model.MimeType { return b.applied }

// selected is the type under the select's cursor (while its list is open,
// the one being looked at).
func (b *body) selected() model.MimeType {
	if b.kind.Cursor < 0 || b.kind.Cursor >= len(b.types) {
		return ""
	}
	return model.MimeType(b.types[b.kind.Cursor])
}

// SetForm switches between the text area (JSON) and form rows
// (urlencoded). Call it before SetValue.
func (b *body) SetForm(form bool) {
	if form {
		b.SetType(model.FormURLEncoded)
	} else {
		b.SetType(model.ApplicationJSON)
	}
}

// SetType sets the body's media type: form rows for forms, JSON colors and
// validation for JSON, plain text for other text, a file for the rest. Call
// it before SetValue.
func (b *body) SetType(t model.MimeType) {
	b.syncLabels(t)
	b.applied = t
	b.mode = modeOf(t)
	b.focusPos = -1
	b.setLang(t)
}

func (b *body) setLang(t model.MimeType) {
	s := strings.ToLower(string(t))
	switch {
	case s == "" || strings.Contains(s, "json"):
		b.editor.Lang = components.LangJSON
	case strings.Contains(s, "javascript"):
		b.editor.Lang = components.LangJS
	default:
		b.editor.Lang = components.LangText
	}
}

// changeType applies a type picked in the select, carrying the content
// over: a JSON object becomes form rows and form rows become JSON.
func (b *body) changeType(t model.MimeType) {
	from := b.mode
	b.syncLabels(t)
	b.applied = t
	b.mode = modeOf(t)
	b.setLang(t)
	switch {
	case from != modeForm && b.mode == modeForm && formJSON(b.grid.rows) == "":
		// The editor's JSON object (kept while the body was a file or
		// nothing) becomes the rows.
		if rows := formRows(b.editor.Value()); len(rows) > 0 {
			b.grid.setRows(append(rows, b.keptRows()...))
		}
	case from == modeForm && b.mode == modeRaw && b.editor.Lang == components.LangJSON:
		if v := formJSON(b.grid.rows); v != "" {
			b.editor.SetValue(v)
		}
	}
	b.focusPos = -1
	if b.focus == focusContent && b.active {
		b.focusOnContent()
	}
}

// keptRows are the rows SetValue keeps: switched off ones and files.
func (b *body) keptRows() []paramField {
	var out []paramField
	for _, r := range b.grid.rows {
		if (!r.enabled || r.file) && r.name.Value() != "" {
			out = append(out, r)
		}
	}
	return out
}

func (b *body) SetActive(active bool) {
	b.active = active
	b.kind.Open = false
	b.editor.Blur()
	b.focusPos = -1
	if !active {
		return
	}
	b.focus = focusContent
	switch b.mode {
	case modeNone:
		b.focus = focusType
	case modeForm:
		b.focusPos = 0
	case modeRaw:
		b.editor.Focus()
	}
}

// focusType moves the focus to the type select (and opens its list).
func (b *body) focusOnType(open bool) {
	b.focus = focusType
	b.editor.Blur()
	b.focusPos = -1
	if open && !b.kind.Open {
		m, _ := b.kind.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		b.kind = m.(components.Selector)
	}
}

func (b *body) focusOnContent() {
	if b.mode == modeNone {
		return
	}
	b.focus = focusContent
	switch b.mode {
	case modeForm:
		if b.focusPos < 0 {
			b.focusPos = 0
		}
	case modeRaw:
		b.editor.Focus()
	}
}

func (b *body) IsActive() bool {
	return b.active
}

// SetValue sets the content; form rows switched off and file rows are kept.
func (b *body) SetValue(s string) {
	b.editor.SetValue(s)
	if b.mode == modeForm {
		b.grid.setRows(append(formRows(s), b.keptRows()...))
	}
}

// SetDisabled replaces the switched-off form fields.
func (b *body) SetDisabled(fields map[string]string) {
	b.replaceRows(func(r paramField) bool { return !r.enabled && !r.file }, fields, false, false)
}

// SetFiles replaces the file rows: files are sent, disabled ones kept.
func (b *body) SetFiles(files, disabled map[string]string) {
	b.replaceRows(func(r paramField) bool { return r.file }, files, true, true)
	b.replaceRows(func(r paramField) bool { return false }, disabled, true, false)
}

// replaceRows drops the rows matching drop and appends fields (sorted) as
// rows, files or not, on or off.
func (b *body) replaceRows(drop func(paramField) bool, fields map[string]string, file, enabled bool) {
	var rows []paramField
	for _, r := range b.grid.rows {
		if drop(r) || (r.name.Value() == "" && r.value.Value() == "" && len(fields) > 0) {
			continue
		}
		rows = append(rows, r)
	}
	names := make([]string, 0, len(fields))
	for k := range fields {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		r := createParamWith(k, fields[k])
		r.enabled = enabled
		r.setFile(file)
		rows = append(rows, r)
	}
	b.grid.setRows(rows)
}

// SetFile sets the binary body's file.
func (b *body) SetFile(path string) { b.file.SetValue(path) }

// Disabled are the switched-off form fields (nil when none or not a form).
func (b *body) Disabled() map[string]string {
	if b.mode != modeForm {
		return nil
	}
	return b.rowsMap(false, false)
}

// Files are the multipart file parts (nil when none or not multipart).
func (b *body) Files() map[string]string { return b.fileRows(true) }

// DisabledFiles are the switched-off file parts, kept in drafts.
func (b *body) DisabledFiles() map[string]string { return b.fileRows(false) }

func (b *body) fileRows(enabled bool) map[string]string {
	if b.mode != modeForm || !b.multipart() {
		return nil
	}
	return b.rowsMap(true, enabled)
}

func (b *body) rowsMap(file, enabled bool) map[string]string {
	m := map[string]string{}
	for _, r := range b.grid.rows {
		if name := strings.TrimSpace(r.name.Value()); name != "" && r.file == file && r.enabled == enabled {
			m[name] = strings.TrimSpace(r.value.Value())
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// File is the binary body's file ("" unless the type is a binary one).
func (b *body) File() string {
	if b.mode != modeBinary {
		return ""
	}
	return strings.TrimSpace(b.file.Value())
}

func (b *body) Value() string {
	switch b.mode {
	case modeRaw:
		return b.editor.Value()
	case modeForm:
		return formJSON(b.grid.rows)
	}
	return ""
}

// Raw reports whether the content is text ($EDITOR can edit it).
func (b *body) Raw() bool { return b.mode == modeRaw }

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
		if k == "" || !r.enabled || r.file {
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
	k := config.DefaultKeyMap
	if b.focus == focusType {
		return []key.Binding{k.Up, k.Down, k.Select, k.Next, k.Back}
	}
	switch b.mode {
	case modeForm:
		bindings := []key.Binding{
			k.Next, k.Prev,
			key.NewBinding(key.WithKeys(k.AddHeader.Keys()...), key.WithHelp(k.AddHeader.Help().Key, "add field")),
			k.Toggle,
		}
		if b.multipart() {
			bindings = append(bindings, k.FilePart)
		}
		return append(bindings, k.BodyType, k.Back)
	case modeBinary:
		return []key.Binding{k.Next, k.BodyType, k.Back}
	}
	return []key.Binding{k.BodyType, k.Back}
}

func (b body) multipart() bool {
	return strings.HasPrefix(strings.ToLower(string(b.Type())), "multipart/")
}

func (b body) Init() tea.Cmd {
	return nil
}

var (
	fileOK      = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Green))
	fileMissing = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Red))
)

func (b body) View() tea.View {
	kind := b.kind
	if b.active && b.focus == focusType {
		kind.Style = kind.Style.BorderForeground(config.DefaultConfig.PrimaryColor())
	}
	head := lipgloss.JoinHorizontal(lipgloss.Center, kind.View().Content, " "+hintStyle.Render(
		config.T("editor.body.type_hint", "type", config.DefaultKeyMap.BodyType.Help().Key)))

	var content string
	switch b.mode {
	case modeNone:
		content = hintStyle.Render("\n " + config.T("editor.body.none_hint", "type", config.DefaultKeyMap.BodyType.Help().Key))
	case modeRaw:
		content = b.editor.View().Content
	case modeForm:
		content = b.formView()
	case modeBinary:
		content = b.fileView()
	}
	view := lipgloss.JoinVertical(lipgloss.Left, head, content)
	if b.kind.Open {
		if dd := b.kind.DropDownView(); dd != "" {
			view = overlay.Composite(dd, view, overlay.Left, overlay.Top, 0, 3)
		}
	}
	return tea.NewView(view)
}

func (b body) formView() string {
	focus := -1
	if b.active && b.focus == focusContent {
		focus = b.focusPos
	}
	k := config.DefaultKeyMap
	hint := config.T("editor.body.form_hint", "add", k.AddHeader.Help().Key)
	if b.multipart() {
		hint = config.T("editor.body.multipart_hint", "add", k.AddHeader.Help().Key, "file", k.FilePart.Help().Key)
	}
	return lipgloss.JoinVertical(lipgloss.Top, section(config.T("editor.body.form_title"), hint+" · "+gridHint(), b.width), b.grid.view(focus))
}

func (b body) fileView() string {
	f := b.file
	f.Style = lipgloss.NewStyle().Width(max(10, b.width))
	f.TextInput.SetWidth(max(1, b.width-2))
	if b.active && b.focus == focusContent {
		f.Style = f.Style.BorderForeground(config.DefaultConfig.PrimaryColor())
	} else {
		f.TextInput.Blur()
	}
	status := ""
	if p := strings.TrimSpace(b.file.Value()); p != "" {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			status = fileOK.Render(" ✓ " + responses.HumanSize(int(info.Size())))
		} else {
			status = fileMissing.Render(" ✗ " + config.T("editor.body.file_missing"))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		section(config.T("editor.body.file_title"), config.T("editor.body.file_hint"), b.width),
		components.Z.Mark(b.fileZone(), f.View().Content),
		status)
}

func (b body) fileZone() string { return b.kind.ID + ":file" }

func (b body) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.editor.SetSize(msg.Width, max(4, msg.Height-6))
		b.width, b.height = msg.Width, msg.Height
		b.grid.setWidth(msg.Width)
		b.kind.Width = min(44, max(20, msg.Width-20))
		return &b, nil

	case tabs.SetActiveTabMsg:
		b.SetActive(msg.Active)
		return &b, nil
	}

	if !b.active {
		return &b, nil
	}
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		return b.click(msg)
	case tea.PasteMsg:
		return b.paste(msg)
	case tea.KeyPressMsg:
		if key.Matches(msg, config.DefaultKeyMap.BodyType) && !b.kind.Open {
			b.focusOnType(true)
			return &b, nil
		}
		if b.kind.Open || b.focus == focusType {
			return b.typeKey(msg)
		}
	}

	switch b.mode {
	case modeForm:
		return b.updateForm(msg)
	case modeBinary:
		return b.updateFile(msg)
	case modeRaw:
		var cmd tea.Cmd
		b.editor, cmd = b.editor.Update(msg)
		return &b, cmd
	}
	return &b, nil
}

// typeKey drives the type select: an open list takes every key; closed,
// up/down change the type at once and tab goes to the content.
func (b body) typeKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if !b.kind.Open {
		switch {
		case key.Matches(k, config.DefaultKeyMap.Next), key.Matches(k, config.DefaultKeyMap.Prev):
			b.focusOnContent()
			return &b, nil
		case key.Matches(k, config.DefaultKeyMap.Back):
			return &b, nil // the tab blurs
		}
	}
	wasOpen := b.kind.Open
	m, _ := b.kind.Update(k)
	b.kind = m.(components.Selector)
	if b.kind.Open {
		return &b, nil // browsing the list: the type changes on a pick
	}
	if t := b.selected(); t != b.applied {
		b.changeType(t)
	}
	if wasOpen && key.Matches(k, config.DefaultKeyMap.Ok) {
		b.focusOnContent() // picked: edit the content
	}
	return &b, nil
}

func (b body) click(c tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if b.kind.Open || b.kind.Zone().InBounds(c) {
		m, _ := b.kind.Update(c)
		b.kind = m.(components.Selector)
		b.focusOnType(false)
		if t := b.selected(); !b.kind.Open && t != b.applied {
			b.changeType(t)
			b.focusOnContent()
		}
		return &b, nil
	}
	switch b.mode {
	case modeForm:
		if cell := b.grid.click(c); cell >= 0 {
			b.focus, b.focusPos = focusContent, cell
		}
	case modeBinary:
		if z := components.Z.Get(b.fileZone()); z.InBounds(c) {
			b.focusOnContent()
			x, _ := z.Pos(c)
			b.file.ClickAt(x)
		}
	case modeRaw:
		if b.editor.Zone().InBounds(c) {
			b.focusOnContent()
		}
		var cmd tea.Cmd
		b.editor, cmd = b.editor.Update(c)
		return &b, cmd
	}
	return &b, nil
}

// paste handles text pasted or files dropped on the tab: a drop on a
// multipart form adds file parts, on a binary body sets its file, on an
// empty body makes it a binary one.
func (b body) paste(p tea.PasteMsg) (tea.Model, tea.Cmd) {
	paths := components.DroppedFiles(p.Content)
	switch {
	case paths != nil && b.mode == modeNone:
		b.changeType(model.OctetStream)
		b.file.SetValue(paths[0])
		b.focusOnContent()
		return &b, notify(config.T("editor.body.dropped_binary", "type", config.DefaultKeyMap.BodyType.Help().Key), components.Info)
	case paths != nil && b.mode == modeBinary:
		b.file.SetValue(paths[0])
		b.focusOnContent()
		if len(paths) > 1 {
			return &b, notify(config.T("editor.body.one_file"), components.Info)
		}
		return &b, nil
	case paths != nil && b.mode == modeForm && b.multipart():
		b.dropFiles(paths)
		return &b, nil
	case paths != nil && b.mode == modeForm:
		return &b, notify(config.T("editor.body.files_need_multipart", "type", config.DefaultKeyMap.BodyType.Help().Key), components.Info)
	}
	if b.focus == focusType {
		b.focusOnContent()
	}
	switch b.mode {
	case modeForm:
		return &b, b.grid.update(b.focusPos, p)
	case modeBinary:
		m, cmd := b.file.Update(p)
		b.file = m.(components.Field)
		return &b, cmd
	case modeRaw:
		if paths != nil {
			p.Content = strings.Join(paths, "\n")
		}
		var cmd tea.Cmd
		b.editor, cmd = b.editor.Update(p)
		return &b, cmd
	}
	return &b, nil
}

// dropFiles makes the focused row a file part with the first path (a new
// row when it holds a text value) and adds a row for each other path.
func (b *body) dropFiles(paths []string) {
	row := -1
	if b.focus == focusContent && b.focusPos >= 0 {
		row = b.focusPos / 2
	}
	if row >= 0 {
		if r := b.grid.rows[row]; !r.file && r.value.Value() != "" {
			row = -1
		}
	}
	base := "file"
	if row >= 0 && strings.TrimSpace(b.grid.rows[row].name.Value()) != "" {
		base = strings.TrimSpace(b.grid.rows[row].name.Value())
	}
	taken := map[string]bool{}
	for i, r := range b.grid.rows {
		if i != row {
			taken[strings.TrimSpace(r.name.Value())] = true
		}
	}
	for i, p := range paths {
		name := base
		for n := 2; taken[name]; n++ {
			name = fmt.Sprintf("%s%d", base, n)
		}
		taken[name] = true
		target := row
		if i > 0 || target < 0 {
			if last := len(b.grid.rows) - 1; i == 0 && b.grid.rows[last].name.Value() == "" && b.grid.rows[last].value.Value() == "" {
				target = last // reuse the blank row
			} else {
				target = b.grid.add() / 2
			}
		}
		r := &b.grid.rows[target]
		r.name.SetValue(name)
		r.value.SetValue(p)
		r.enabled = true
		r.setFile(true)
		b.focus, b.focusPos = focusContent, target*2+1
	}
}

func (b body) updateFile(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		if b.file.Completing() && menuKey(k) {
			m, cmd := b.file.Update(k)
			b.file = m.(components.Field)
			return &b, cmd
		}
		if key.Matches(k, config.DefaultKeyMap.Next) || key.Matches(k, config.DefaultKeyMap.Prev) {
			b.focusOnType(false)
			return &b, nil
		}
		m, cmd := b.file.Update(k)
		b.file = m.(components.Field)
		return &b, cmd
	}
	return &b, nil
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
	case key.Matches(k, config.DefaultKeyMap.Toggle):
		b.grid.toggle(b.focusPos)
	case key.Matches(k, config.DefaultKeyMap.FilePart) && b.multipart():
		if b.focusPos >= 0 && b.focusPos < total {
			r := &b.grid.rows[b.focusPos/2]
			r.setFile(!r.file)
		}
	case key.Matches(k, config.DefaultKeyMap.Next):
		if b.focusPos == total-1 {
			b.focusOnType(false) // wraps through the type select
			return &b, nil
		}
		b.focusPos++
	case key.Matches(k, config.DefaultKeyMap.Prev):
		if b.focusPos <= 0 {
			b.focusOnType(false)
			return &b, nil
		}
		b.focusPos--
	default:
		return &b, b.grid.update(b.focusPos, k)
	}
	return &b, nil
}

// Completing reports whether a completion menu or the type list is open
// (esc, tab and the arrows belong to it).
func (b body) Completing() bool {
	if !b.active {
		return false
	}
	if b.kind.Open {
		return true
	}
	if b.focus == focusType {
		return false
	}
	switch b.mode {
	case modeForm:
		return b.grid.completing(b.focusPos)
	case modeBinary:
		return b.file.Completing()
	case modeRaw:
		return b.editor.Completing()
	}
	return false
}

// HasSelection reports whether ctrl+c copies: text selected in the editor,
// or a focused field with a value.
func (b body) HasSelection() bool {
	if !b.active || b.focus != focusContent {
		return false
	}
	switch b.mode {
	case modeRaw:
		return b.editor.HasSelection()
	case modeForm:
		return b.grid.canCopy(b.focusPos)
	case modeBinary:
		return b.file.CanCopy()
	}
	return false
}
