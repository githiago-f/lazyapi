package editor

import (
	"strconv"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/oauth"
)

// LoginMsg asks the TUI to log in to an OAuth2 / OpenID Connect scheme
// (the Auth tab's Login button or ctrl+l).
type LoginMsg struct {
	Spec   string
	Scheme model.AuthScheme
}

// newScheme is the label of the Scheme select that defines a new scheme
// (the others reuse a scheme of the spec's components).
var newScheme = config.T("editor.auth.new_scheme")

// label and placeholder are the Auth tab's texts (messages.yml).
func label(name string) string       { return config.T("editor.auth.labels." + name) }
func placeholder(name string) string { return config.T("editor.auth.placeholders." + name) }

var keyInLabels = []string{"header", "query", "cookie"}

// authScheme is one security scheme of the request. Its rows depend on the
// type and the grant (see rows).
type authScheme struct {
	zone string // mouse zone prefix

	schemeSelector    components.Selector // reuse a spec scheme, or new
	nameField         components.Field
	typeSelector      components.Selector
	usernameField     components.Field
	passwordField     components.PassField
	tokenField        components.Field
	keyNameField      components.Field
	keyInSelector     components.Selector
	keyValueField     components.Field
	grantTypeSelector components.Selector
	issuerField       components.Field
	clientIDField     components.Field
	clientSecretField components.PassField
	authURLField      components.Field
	tokenURLField     components.Field
	scopesField       components.Field
	redirectField     components.Field
	loginButton       components.Button

	params  map[string]string // kept as loaded (not editable here)
	reused  string            // the spec scheme last copied in
	enabled bool              // switched off: kept, not applied
}

// authRow is one focusable line: a select, a text field or the button.
type authRow struct {
	label  string
	sel    *components.Selector
	field  *components.Field
	pass   *components.PassField
	button *components.Button
}

func authTypeLabels() []string {
	labels := make([]string, len(model.AuthTypes))
	for i, t := range model.AuthTypes {
		labels[i] = t.Label()
	}
	return labels
}

func newAuthScheme(known []string) authScheme {
	zone := components.Z.NewPrefix()
	sel := func(id string, labels []string, w int) components.Selector {
		return components.Selector{Labels: labels, Width: w, ID: zone + id}
	}
	return authScheme{
		zone:              zone,
		schemeSelector:    sel("scheme", append([]string{newScheme}, known...), 24),
		nameField:         components.InitField(placeholder("name"), ""),
		typeSelector:      sel("type", authTypeLabels(), 20),
		usernameField:     components.InitField(placeholder("username"), ""),
		passwordField:     components.InitPassField(placeholder("password"), "", false),
		tokenField:        components.InitField(placeholder("token"), ""),
		keyNameField:      components.InitField(placeholder("key_name"), ""),
		keyInSelector:     sel("in", keyInLabels, 12),
		keyValueField:     components.InitField(placeholder("key_value"), ""),
		grantTypeSelector: sel("grant", oauth.Grants, 22),
		issuerField:       components.InitField(placeholder("issuer"), ""),
		clientIDField:     components.InitField(placeholder("client_id"), ""),
		clientSecretField: components.InitPassField(placeholder("client_secret"), "", false),
		authURLField:      components.InitField(placeholder("auth_url"), ""),
		tokenURLField:     components.InitField(placeholder("token_url"), ""),
		scopesField:       components.InitField(placeholder("scopes"), ""),
		redirectField:     components.InitField(oauth.DefaultRedirectURI, ""),
		loginButton:       components.Button{Label: config.T("editor.auth.login"), Active: true, ID: zone + "login"},
		reused:            newScheme,
		enabled:           true,
	}
}

func (as *authScheme) authType() model.AuthType {
	if as.typeSelector.Cursor >= 0 && as.typeSelector.Cursor < len(model.AuthTypes) {
		return model.AuthTypes[as.typeSelector.Cursor]
	}
	return model.AuthBasic
}

func (as *authScheme) grant() string { return as.grantTypeSelector.Value() }

// isNew reports whether the scheme is defined here (not reused).
func (as *authScheme) isNew() bool { return as.schemeSelector.Cursor == 0 }

// rows are the focusable lines for the current type and grant.
func (as *authScheme) rows() []authRow {
	rows := []authRow{{label: label("scheme"), sel: &as.schemeSelector}}
	if as.isNew() {
		rows = append(rows, authRow{label: label("name"), field: &as.nameField})
	}
	rows = append(rows, authRow{label: label("type"), sel: &as.typeSelector})
	f := func(label string, field *components.Field) authRow { return authRow{label: label, field: field} }
	p := func(label string, field *components.PassField) authRow {
		return authRow{label: label, field: &field.Field, pass: field}
	}
	switch t := as.authType(); t {
	case model.AuthBasic:
		rows = append(rows, f(label("username"), &as.usernameField), p(label("password"), &as.passwordField))
	case model.AuthBearer:
		rows = append(rows, f(label("token"), &as.tokenField))
	case model.AuthAPIKey:
		rows = append(rows, f(label("key_name"), &as.keyNameField), authRow{label: label("key_in"), sel: &as.keyInSelector}, f(label("key_value"), &as.keyValueField))
	case model.AuthOAuth2, model.AuthOpenIDConnect:
		if t == model.AuthOpenIDConnect {
			rows = append(rows, f(label("issuer"), &as.issuerField))
		}
		rows = append(rows, authRow{label: label("grant"), sel: &as.grantTypeSelector},
			f(label("client_id"), &as.clientIDField), p(label("client_secret"), &as.clientSecretField))
		g := as.grant()
		if t == model.AuthOAuth2 {
			if g == oauth.GrantAuthorizationCode || g == oauth.GrantImplicit {
				rows = append(rows, f(label("auth_url"), &as.authURLField))
			}
			if g != oauth.GrantImplicit {
				rows = append(rows, f(label("token_url"), &as.tokenURLField))
			}
		}
		rows = append(rows, f(label("scopes"), &as.scopesField))
		switch g {
		case oauth.GrantAuthorizationCode:
			rows = append(rows, f(label("redirect"), &as.redirectField))
		case oauth.GrantPassword:
			rows = append(rows, f(label("username"), &as.usernameField), p(label("password"), &as.passwordField))
		}
		if g != oauth.GrantImplicit {
			rows = append(rows, authRow{label: "", button: &as.loginButton})
		}
	}
	return rows
}

func (as *authScheme) fieldCount() int { return len(as.rows()) }

func (as *authScheme) row(i int) (authRow, bool) {
	rows := as.rows()
	if i < 0 || i >= len(rows) {
		return authRow{}, false
	}
	return rows[i], true
}

// openSelector is the selector of the scheme whose list is open (nil: none).
func (as *authScheme) openSelector() *components.Selector {
	for _, r := range as.rows() {
		if r.sel != nil && r.sel.Open {
			return r.sel
		}
	}
	return nil
}

// updateField sends msg to row i. pressed reports a Login press.
func (as *authScheme) updateField(i int, msg tea.Msg) (cmd tea.Cmd, pressed bool) {
	r, ok := as.row(i)
	if !ok {
		return nil, false
	}
	switch {
	case r.sel != nil:
		m, c := r.sel.Update(msg)
		*r.sel = m.(components.Selector)
		return c, false
	case r.pass != nil:
		m, c := r.pass.Update(msg)
		*r.pass = m.(components.PassField)
		return c, false
	case r.field != nil:
		m, c := r.field.Update(msg)
		*r.field = m.(components.Field)
		return c, false
	case r.button != nil:
		if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, config.DefaultKeyMap.Select) {
			m, c := r.button.Update(msg)
			*r.button = m.(components.Button)
			return c, true
		}
		m, c := r.button.Update(msg)
		*r.button = m.(components.Button)
		return c, false
	}
	return nil, false
}

// textField is the text field at row i (nil for selects and the button).
func (as *authScheme) textField(i int) *components.Field {
	if r, ok := as.row(i); ok {
		return r.field
	}
	return nil
}

func (as *authScheme) rowZone(i int) string { return as.zone + "row" + strconv.Itoa(i) }
func (as *authScheme) boxZone() string      { return as.zone + "chk" }

// header is the scheme's first line: its on/off box and what it is.
func (as *authScheme) header(width int) string {
	box, st := config.T("editor.auth.switch_off"), checkOff
	if as.enabled {
		box, st = config.T("editor.auth.switch_on"), checkOn
	}
	name := as.nameField.Value()
	if !as.isNew() {
		name = as.schemeSelector.Value()
	}
	if name == "" {
		name = config.T("editor.auth.unnamed")
	}
	desc := labelStyle.Render(" " + name + " · " + as.authType().Label())
	if !as.enabled {
		desc += sessionStyle.Render(" — " + config.T("editor.auth.off", "toggle", config.DefaultKeyMap.Toggle.Help().Key))
	}
	return ansi.Truncate(components.Z.Mark(as.boxZone(), st.Render(box))+desc, width, "…")
}

var (
	sessionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Subtext0)).Italic(true)
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(config.Subtext1))
)

func (as *authScheme) viewFields(width int, isActive bool, active int, status string) string {
	var lines []string
	for i, r := range as.rows() {
		focused := isActive && active == i
		var block string
		switch {
		case r.sel != nil:
			block = renderSelector(r.label, r.sel, width, focused)
		case r.button != nil:
			b := *r.button
			b.Focused = focused
			block = lipgloss.JoinHorizontal(lipgloss.Center, rowLabel("", false), b.View().Content)
			if room := width - lipgloss.Width(block) - 1; status != "" && room > 4 {
				block = lipgloss.JoinHorizontal(lipgloss.Center, block, " ", sessionStyle.Render(ansi.Truncate(status, room, "…")))
			}
		default:
			block = renderField(r.label, r.field, width, focused)
		}
		lines = append(lines, components.Z.Mark(as.rowZone(i), block))
	}
	return lipgloss.JoinVertical(lipgloss.Top, lines...)
}

// labelWidth is the label column of the Auth tab's rows.
const labelWidth = 15

func rowLabel(label string, focused bool) string {
	st := labelStyle.Width(labelWidth).PaddingTop(1)
	if focused {
		st = st.Foreground(config.DefaultConfig.PrimaryColor()).Bold(true)
	}
	return st.Render(label)
}

// renderField draws a labelled field; the label sits left of the box.
func renderField(label string, f *components.Field, width int, focused bool) string {
	w := max(8, width-labelWidth)
	f.Style = lipgloss.NewStyle().Width(w).UnsetBorderForeground()
	f.TextInput.SetWidth(max(0, w-2))
	if focused {
		f.Style = f.Style.BorderForeground(config.DefaultConfig.PrimaryColor())
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, rowLabel(label, focused), f.View().Content)
	if menu := f.MenuView(w); menu != "" && focused {
		row = lipgloss.JoinVertical(lipgloss.Left, row, lipgloss.NewStyle().PaddingLeft(labelWidth).Render(menu))
	}
	return row
}

// renderSelector draws a select; its open list is drawn inline below it.
func renderSelector(label string, s *components.Selector, width int, focused bool) string {
	s.Style = lipgloss.NewStyle().UnsetBorderForeground()
	if focused {
		s.Style = s.Style.BorderForeground(config.DefaultConfig.PrimaryColor())
	}
	s.Width = min(max(8, width-labelWidth), 28)
	row := lipgloss.JoinHorizontal(lipgloss.Top, rowLabel(label, focused), s.View().Content)
	if dd := s.DropDownView(); dd != "" {
		row = lipgloss.JoinVertical(lipgloss.Left, row, lipgloss.NewStyle().PaddingLeft(labelWidth).Render(dd))
	}
	return row
}

type auth struct {
	active   bool
	width    int
	focusPos int
	schemes  []authScheme

	spec  string             // the request's spec (token store, login)
	known []model.AuthScheme // the spec's security schemes (reusable)
	// Sessions describes a scheme's stored OAuth session (set by the TUI).
	Sessions func(spec string, s model.AuthScheme) string
}

func AuthorizeTab() *auth {
	return &auth{focusPos: -1}
}

func (a *auth) SetActive(b bool) {
	a.active = b
	a.focusPos = -1
	if b && len(a.schemes) > 0 {
		a.focusPos = 0
	}
	if !b {
		for i := range a.schemes {
			if s := a.schemes[i].openSelector(); s != nil {
				s.Open = false
			}
		}
	}
}

func (a *auth) IsActive() bool { return a.active }

func (a auth) Init() tea.Cmd { return nil }

func (a *auth) totalFields() int {
	total := 0
	for i := range a.schemes {
		total += a.schemes[i].fieldCount()
	}
	return total
}

func (a *auth) resolveField(pos int) (int, int) {
	for i := range a.schemes {
		cnt := a.schemes[i].fieldCount()
		if pos < cnt {
			return i, pos
		}
		pos -= cnt
	}
	return 0, 0
}

func (a *auth) offset(si int) int {
	n := 0
	for i := range si {
		n += a.schemes[i].fieldCount()
	}
	return n
}

func (a auth) View() tea.View {
	if len(a.schemes) == 0 {
		return tea.NewView(hintStyle.Render(config.T("editor.auth.empty", "add", config.DefaultKeyMap.AddAuth.Help().Key)))
	}

	schemeStyle := lipgloss.NewStyle().
		Width(a.width).
		Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(config.Surface2)).
		Padding(0, 1)

	var rendered []string
	base := 0
	for si := range a.schemes {
		as := &a.schemes[si]
		fc := as.fieldCount()
		isFocused := a.active && a.focusPos >= base && a.focusPos < base+fc
		rel := -1
		if isFocused {
			rel = a.focusPos - base
		}
		base += fc

		status := ""
		if as.authType().IsOAuth() && a.Sessions != nil {
			status = a.Sessions(a.spec, as.value())
		}
		s := schemeStyle
		if isFocused {
			s = s.BorderForeground(config.DefaultConfig.PrimaryColor())
		}
		body := lipgloss.JoinVertical(lipgloss.Left, as.header(max(0, a.width-4)), as.viewFields(max(0, a.width-4), isFocused, rel, status))
		rendered = append(rendered, s.Render(body))
	}
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Top, rendered...))
}

// knownNames are the spec schemes offered by the Scheme select.
func (a *auth) knownNames() []string {
	names := make([]string, len(a.known))
	for i, k := range a.known {
		names[i] = k.SchemeName
	}
	return names
}

// SetSpec sets the request's spec and the schemes it defines.
func (a *auth) SetSpec(spec string, known []model.AuthScheme) {
	a.spec, a.known = spec, known
}

func (a *auth) SetValue(schemes []model.AuthScheme) {
	a.schemes = make([]authScheme, len(schemes))
	for i, s := range schemes {
		as := newAuthScheme(a.knownNames())
		as.load(s)
		if !as.schemeSelector.SetValue(s.SchemeName) {
			as.schemeSelector.Cursor = 0
			as.nameField.SetValue(s.SchemeName)
		}
		as.reused = as.schemeSelector.Value() // keep the request's own values
		a.schemes[i] = as
	}
	a.focusPos = -1
	if a.active && len(a.schemes) > 0 {
		a.focusPos = 0
	}
}

// load fills the fields from a scheme (the Scheme select is left alone).
func (as *authScheme) load(s model.AuthScheme) {
	for i, t := range model.AuthTypes {
		if t == s.Type {
			as.typeSelector.Cursor = i
		}
	}
	as.nameField.SetValue(s.SchemeName)
	as.usernameField.SetValue(s.Username)
	as.passwordField.SetValue(s.Password)
	as.tokenField.SetValue(s.Token)
	as.keyNameField.SetValue(s.KeyName)
	as.keyValueField.SetValue(s.KeyValue)
	as.issuerField.SetValue(s.Issuer)
	as.clientIDField.SetValue(s.ClientID)
	as.clientSecretField.SetValue(s.ClientSecret)
	as.authURLField.SetValue(s.AuthURL)
	as.tokenURLField.SetValue(s.TokenURL)
	as.scopesField.SetValue(s.Scopes)
	as.redirectField.SetValue(s.RedirectURI)
	as.params = s.Params
	as.enabled = !s.Disabled
	as.keyInSelector.Cursor = 0
	as.keyInSelector.SetValue(s.KeyIn)
	as.grantTypeSelector.Cursor = 0
	as.grantTypeSelector.SetValue(oauth.NormalizeGrant(s.GrantType))
}

func (as *authScheme) value() model.AuthScheme {
	name := as.nameField.Value()
	if !as.isNew() {
		name = as.schemeSelector.Value()
	}
	s := model.AuthScheme{
		Type:         as.authType(),
		SchemeName:   name,
		Username:     as.usernameField.Value(),
		Password:     as.passwordField.Value(),
		Token:        as.tokenField.Value(),
		KeyName:      as.keyNameField.Value(),
		KeyIn:        as.keyInSelector.Value(),
		KeyValue:     as.keyValueField.Value(),
		GrantType:    as.grantTypeSelector.Value(),
		Issuer:       as.issuerField.Value(),
		ClientID:     as.clientIDField.Value(),
		ClientSecret: as.clientSecretField.Value(),
		AuthURL:      as.authURLField.Value(),
		TokenURL:     as.tokenURLField.Value(),
		Scopes:       as.scopesField.Value(),
		RedirectURI:  as.redirectField.Value(),
		Params:       as.params,
		Disabled:     !as.enabled,
	}
	if !s.Type.IsOAuth() {
		s.GrantType = ""
	}
	if s.Type != model.AuthAPIKey {
		s.KeyIn = ""
	}
	return s
}

func (a *auth) Value() []model.AuthScheme {
	var schemes []model.AuthScheme
	for i := range a.schemes {
		schemes = append(schemes, a.schemes[i].value())
	}
	return schemes
}

func (a auth) HelpBindings() []key.Binding {
	return []key.Binding{
		config.DefaultKeyMap.Next,
		config.DefaultKeyMap.Prev,
		config.DefaultKeyMap.AddAuth,
		config.DefaultKeyMap.DelAuth,
		config.DefaultKeyMap.Toggle,
		config.DefaultKeyMap.Login,
		config.DefaultKeyMap.Back,
	}
}

// reuse copies the definition of the spec scheme picked in the Scheme
// select (once per pick: edits made afterwards are kept).
func (a *auth) reuse(si int) {
	as := &a.schemes[si]
	if as.schemeSelector.Open || as.reused == as.schemeSelector.Value() {
		return
	}
	as.reused = as.schemeSelector.Value()
	if as.isNew() {
		return
	}
	for _, k := range a.known {
		if k.SchemeName == as.reused {
			as.load(k)
			return
		}
	}
}

func (a *auth) login(si int) tea.Cmd {
	msg := LoginMsg{Spec: a.spec, Scheme: a.schemes[si].value()}
	return func() tea.Msg { return msg }
}

func (a auth) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width

	case tabs.SetActiveTabMsg:
		a.SetActive(msg.Active)
		return &a, nil

	case tea.MouseClickMsg:
		if !a.active || msg.Button != tea.MouseLeft {
			return &a, nil
		}
		return &a, a.click(msg)

	case tea.KeyPressMsg:
		if !a.active {
			return &a, nil
		}
		return &a, a.key(msg)

	default:
		// button flash timers
		if a.focusPos >= 0 && len(a.schemes) > 0 {
			si, fi := a.resolveField(a.focusPos)
			cmd, _ := a.schemes[si].updateField(fi, msg)
			return &a, cmd
		}
	}
	return &a, nil
}

func (a *auth) key(msg tea.KeyPressMsg) tea.Cmd {
	if key.Matches(msg, config.DefaultKeyMap.AddAuth) {
		a.schemes = append(a.schemes, newAuthScheme(a.knownNames()))
		a.focusPos = a.offset(len(a.schemes) - 1)
		return nil
	}
	if a.focusPos < 0 || len(a.schemes) == 0 {
		return nil
	}
	si, fi := a.resolveField(a.focusPos)
	as := &a.schemes[si]

	// An open list owns every key until it closes.
	if s := as.openSelector(); s != nil {
		m, _ := s.Update(msg)
		*s = m.(components.Selector)
		a.reuse(si)
		a.clampFocus()
		return nil
	}
	// An open completion menu owns tab/esc/arrows.
	if f := as.textField(fi); f != nil && f.Completing() && menuKey(msg) {
		cmd, _ := as.updateField(fi, msg)
		return cmd
	}
	if key.Matches(msg, config.DefaultKeyMap.Login) && as.authType().IsOAuth() {
		return a.login(si)
	}
	if key.Matches(msg, config.DefaultKeyMap.Toggle) {
		as.enabled = !as.enabled
		return nil
	}

	total := a.totalFields()
	switch {
	case fi == 0 && key.Matches(msg, config.DefaultKeyMap.DelAuth):
		a.schemes = append(a.schemes[:si], a.schemes[si+1:]...)
		a.focusPos = -1
		if len(a.schemes) > 0 {
			a.focusPos = max(0, min(a.focusPos, a.totalFields()-1))
		}
	case key.Matches(msg, config.DefaultKeyMap.Next):
		a.focusPos = (a.focusPos + 1) % total
	case key.Matches(msg, config.DefaultKeyMap.Prev):
		a.focusPos = (a.focusPos - 1 + total) % total
	default:
		cmd, pressed := as.updateField(fi, msg)
		a.reuse(si)
		a.clampFocus()
		if pressed {
			return tea.Batch(cmd, a.login(si))
		}
		return cmd
	}
	return nil
}

// click focuses the row under the mouse; selects open (or pick from their
// open list), fields place the cursor, the Login button logs in.
func (a *auth) click(msg tea.MouseClickMsg) tea.Cmd {
	// An open list takes the click first (a pick, or a click outside).
	for si := range a.schemes {
		as := &a.schemes[si]
		if s := as.openSelector(); s != nil {
			m, _ := s.Update(msg)
			*s = m.(components.Selector)
			a.reuse(si)
			a.clampFocus()
			return nil
		}
	}
	for si := range a.schemes {
		as := &a.schemes[si]
		if components.Z.Get(as.boxZone()).InBounds(msg) {
			as.enabled = !as.enabled
			a.focusPos = a.offset(si)
			return nil
		}
		for i, r := range as.rows() {
			z := components.Z.Get(as.rowZone(i))
			if !z.InBounds(msg) {
				continue
			}
			a.focusPos = a.offset(si) + i
			switch {
			case r.sel != nil:
				m, _ := r.sel.Update(msg)
				*r.sel = m.(components.Selector)
			case r.button != nil:
				if r.button.Zone().InBounds(msg) {
					m, cmd := r.button.Update(msg)
					*r.button = m.(components.Button)
					return tea.Batch(cmd, a.login(si))
				}
			case r.field != nil:
				// the label column sits left of the box
				x, _ := components.Z.Get(as.rowZone(i)).Pos(msg)
				r.field.ClickAt(x - labelWidth)
			}
			return nil
		}
	}
	return nil
}

// clampFocus keeps the focus valid after rows appear or disappear.
func (a *auth) clampFocus() {
	if total := a.totalFields(); a.focusPos >= total {
		a.focusPos = total - 1
	}
}

// Completing reports whether the focused field has its menu open or a
// select shows its list (esc and tab then belong to it).
func (a auth) Completing() bool {
	if !a.active || a.focusPos < 0 || len(a.schemes) == 0 {
		return false
	}
	si, fi := a.resolveField(a.focusPos)
	if a.schemes[si].openSelector() != nil {
		return true
	}
	f := a.schemes[si].textField(fi)
	return f != nil && f.Completing()
}

// SelectOpen reports whether a select of the tab shows its list.
func (a auth) SelectOpen() bool {
	for i := range a.schemes {
		if a.schemes[i].openSelector() != nil {
			return true
		}
	}
	return false
}
