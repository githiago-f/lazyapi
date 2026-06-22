package editor

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/model"
)

type authScheme struct {
	enabled      bool
	typeSelector components.Selector

	usernameField     components.Field
	passwordField     components.PassField
	tokenField        components.Field
	keyNameField      components.Field
	keyInSelector     components.Selector
	keyValueField     components.Field
	grantTypeSelector components.Selector
	clientIDField     components.Field
	clientSecretField components.PassField
	authURLField      components.Field
	tokenURLField     components.Field
	scopesField       components.Field

	schemeName string
}

func newAuthScheme() authScheme {
	return authScheme{
		enabled: true,
		typeSelector: components.Selector{
			Cursor: 0,
			Labels: authTypeLabels(),
			Width:  16,
		},
		usernameField: components.InitField("Username", ""),
		passwordField: components.InitPassField("Password", "", false),
		tokenField:    components.InitField("Token", ""),
		keyNameField:  components.InitField("Key Name", ""),
		keyInSelector: components.Selector{
			Cursor: 0,
			Labels: []string{"header", "query", "cookie"},
			Width:  8,
		},
		keyValueField: components.InitField("Value", ""),
		grantTypeSelector: components.Selector{
			Cursor: 0,
			Labels: []string{"authorizationCode", "clientCredentials", "implicit", "password"},
			Width:  20,
		},
		clientIDField:     components.InitField("Client ID", ""),
		clientSecretField: components.InitPassField("Client Secret", "", false),
		authURLField:      components.InitField("Auth URL", ""),
		tokenURLField:     components.InitField("Token URL", ""),
		scopesField:       components.InitField("Scopes", ""),
	}
}

func authTypeLabels() []string {
	return []string{
		model.AuthBasic.Label(),
		model.AuthBearer.Label(),
		model.AuthAPIKey.Label(),
		model.AuthOAuth2.Label(),
	}
}

func (as *authScheme) fieldCount() int {
	n := 1 // type selector
	switch as.typeSelector.Cursor {
	case int(model.AuthBasic):
		n += 2
	case int(model.AuthBearer):
		n += 1
	case int(model.AuthAPIKey):
		n += 3
	case int(model.AuthOAuth2):
		n += 6
	}
	return n
}

func (as authScheme) updateField(fieldIdx int, msg tea.Msg) (authScheme, tea.Cmd) {
	if fieldIdx == 0 {
		m, c := as.typeSelector.Update(msg)
		as.typeSelector = m.(components.Selector)
		return as, c
	}
	inner := fieldIdx - 1
	switch as.typeSelector.Cursor {
	case int(model.AuthBasic):
		switch inner {
		case 0:
			m, c := as.usernameField.Update(msg)
			as.usernameField = m.(components.Field)
			return as, c
		case 1:
			m, c := as.passwordField.Update(msg)
			as.passwordField = m.(components.PassField)
			return as, c
		}
	case int(model.AuthBearer):
		if inner == 0 {
			m, c := as.tokenField.Update(msg)
			as.tokenField = m.(components.Field)
			return as, c
		}
	case int(model.AuthAPIKey):
		switch inner {
		case 0:
			m, c := as.keyNameField.Update(msg)
			as.keyNameField = m.(components.Field)
			return as, c
		case 1:
			m, c := as.keyInSelector.Update(msg)
			as.keyInSelector = m.(components.Selector)
			return as, c
		case 2:
			m, c := as.keyValueField.Update(msg)
			as.keyValueField = m.(components.Field)
			return as, c
		}
	case int(model.AuthOAuth2):
		switch inner {
		case 0:
			m, c := as.grantTypeSelector.Update(msg)
			as.grantTypeSelector = m.(components.Selector)
			return as, c
		case 1:
			m, c := as.clientIDField.Update(msg)
			as.clientIDField = m.(components.Field)
			return as, c
		case 2:
			m, c := as.clientSecretField.Update(msg)
			as.clientSecretField = m.(components.PassField)
			return as, c
		case 3:
			m, c := as.authURLField.Update(msg)
			as.authURLField = m.(components.Field)
			return as, c
		case 4:
			m, c := as.tokenURLField.Update(msg)
			as.tokenURLField = m.(components.Field)
			return as, c
		case 5:
			m, c := as.scopesField.Update(msg)
			as.scopesField = m.(components.Field)
			return as, c
		}
	}
	return as, nil
}

func (as *authScheme) viewFields(width int, isActive bool, activeFieldIdx int) string {
	var lines []string

	lines = append(lines, renderSelector("Type", &as.typeSelector, width, isActive && activeFieldIdx == 0))

	switch as.typeSelector.Cursor {
	case int(model.AuthBasic):
		lines = append(lines, renderField("Username", &as.usernameField, width, isActive && activeFieldIdx == 1))
		lines = append(lines, renderField("Password", &as.passwordField.Field, width, isActive && activeFieldIdx == 2))
	case int(model.AuthBearer):
		lines = append(lines, renderField("Token", &as.tokenField, width, isActive && activeFieldIdx == 1))
	case int(model.AuthAPIKey):
		lines = append(lines, renderField("Key Name", &as.keyNameField, width, isActive && activeFieldIdx == 1))
		lines = append(lines, renderSelector("In", &as.keyInSelector, width, isActive && activeFieldIdx == 2))
		lines = append(lines, renderField("Value", &as.keyValueField, width, isActive && activeFieldIdx == 3))
	case int(model.AuthOAuth2):
		lines = append(lines, renderSelector("Grant Type", &as.grantTypeSelector, width, isActive && activeFieldIdx == 1))
		lines = append(lines, renderField("Client ID", &as.clientIDField, width, isActive && activeFieldIdx == 2))
		lines = append(lines, renderField("Client Secret", &as.clientSecretField.Field, width, isActive && activeFieldIdx == 3))
		lines = append(lines, renderField("Auth URL", &as.authURLField, width, isActive && activeFieldIdx == 4))
		lines = append(lines, renderField("Token URL", &as.tokenURLField, width, isActive && activeFieldIdx == 5))
		lines = append(lines, renderField("Scopes", &as.scopesField, width, isActive && activeFieldIdx == 6))
	}

	return lipgloss.JoinVertical(lipgloss.Top, lines...)
}

func renderField(label string, f *components.Field, width int, focused bool) string {
	activeColor := config.DefaultConfig.PrimaryColor()
	f.Style = lipgloss.NewStyle().Width(width - len(label) - 2)
	if focused {
		f.Style = f.Style.BorderForeground(activeColor)
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		label,
		f.View(),
	)
}

func renderSelector(label string, s *components.Selector, width int, focused bool) string {
	activeColor := config.DefaultConfig.PrimaryColor()
	s.Style = lipgloss.NewStyle().Width(width - len(label) - 2)
	if focused {
		s.Style = s.Style.BorderForeground(activeColor)
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		label,
		s.View(),
	)
}

type auth struct {
	active   bool
	width    int
	focusPos int
	schemes  []authScheme
}

func AuthorizeTab() *auth {
	return &auth{
		focusPos: -1,
	}
}

func (a *auth) SetActive(b bool) {
	a.active = b
	if b {
		if len(a.schemes) > 0 {
			a.focusPos = 0
		} else {
			a.focusPos = -1
		}
	} else {
		a.focusPos = -1
	}
}

func (a *auth) IsActive() bool {
	return a.active
}

func (a auth) Init() tea.Cmd {
	return nil
}

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

func (a auth) View() string {
	if len(a.schemes) == 0 {
		return "No auth schemes defined. Press A to add one."
	}

	activeColor := config.DefaultConfig.PrimaryColor()
	schemeStyle := lipgloss.NewStyle().
		Width(a.width-2).
		Border(lipgloss.NormalBorder()).
		Padding(0, 1)

	fieldBaseOffset := 0
	var rendered []string

	for si := range a.schemes {
		fc := a.schemes[si].fieldCount()
		isFocused := a.active && a.focusPos >= fieldBaseOffset && a.focusPos < fieldBaseOffset+fc
		relFocus := -1
		if isFocused {
			relFocus = a.focusPos - fieldBaseOffset
		}
		fieldBaseOffset += fc

		indicator := "( )"
		if a.schemes[si].enabled {
			indicator = "(o)"
		}
		label := fmt.Sprintf("Scheme %d", si+1)
		if a.schemes[si].schemeName != "" {
			label = a.schemes[si].schemeName
		}

		schemeContent := checkStyle.Render(indicator) + " " + label + "\n" + a.schemes[si].viewFields(a.width-2, isFocused, relFocus)

		s := schemeStyle
		if isFocused {
			s = s.BorderForeground(activeColor)
		}
		rendered = append(rendered, s.Render(schemeContent))
	}

	return lipgloss.JoinVertical(lipgloss.Top, rendered...)
}

func (a *auth) SetValue(schemes []model.AuthScheme) {
	a.SetValueWithEnabled(schemes, nil)
}

func (a *auth) SetValueWithEnabled(schemes []model.AuthScheme, enabled []bool) {
	a.schemes = make([]authScheme, len(schemes))
	for i, s := range schemes {
		as := newAuthScheme()
		as.schemeName = s.SchemeName
		as.typeSelector.Cursor = int(s.Type)
		as.usernameField.SetValue(s.Username)
		as.passwordField.SetValue(s.Password)
		as.tokenField.SetValue(s.Token)
		as.keyNameField.SetValue(s.KeyName)
		as.keyValueField.SetValue(s.KeyValue)
		as.clientIDField.SetValue(s.ClientID)
		as.clientSecretField.SetValue(s.ClientSecret)
		as.authURLField.SetValue(s.AuthURL)
		as.tokenURLField.SetValue(s.TokenURL)
		as.scopesField.SetValue(s.Scopes)

		if enabled != nil && i < len(enabled) {
			as.enabled = enabled[i]
		}

		for gi, gl := range []string{"header", "query", "cookie"} {
			if gl == s.KeyIn {
				as.keyInSelector.Cursor = gi
				break
			}
		}

		for gi, gl := range []string{"authorizationCode", "clientCredentials", "implicit", "password"} {
			if gl == s.GrantType {
				as.grantTypeSelector.Cursor = gi
				break
			}
		}

		a.schemes[i] = as
	}
	if a.active {
		a.focusPos = 0
	}
}

func (a *auth) EnabledAuth() []bool {
	r := make([]bool, len(a.schemes))
	for i, s := range a.schemes {
		r[i] = s.enabled
	}
	return r
}

func (a *auth) Value() []model.AuthScheme {
	var schemes []model.AuthScheme
	for _, as := range a.schemes {
		if !as.enabled {
			continue
		}
		kt := authTypeLabels()[as.typeSelector.Cursor]
		var at model.AuthType
		for _, t := range []model.AuthType{model.AuthBasic, model.AuthBearer, model.AuthAPIKey, model.AuthOAuth2} {
			if t.Label() == kt {
				at = t
				break
			}
		}

		keyIn := ""
		if as.keyInSelector.Cursor >= 0 && as.keyInSelector.Cursor < len(as.keyInSelector.Labels) {
			keyIn = as.keyInSelector.Labels[as.keyInSelector.Cursor]
		}
		grantType := ""
		if as.grantTypeSelector.Cursor >= 0 && as.grantTypeSelector.Cursor < len(as.grantTypeSelector.Labels) {
			grantType = as.grantTypeSelector.Labels[as.grantTypeSelector.Cursor]
		}

		schemes = append(schemes, model.AuthScheme{
			Type:         at,
			SchemeName:   as.schemeName,
			Username:     as.usernameField.Value(),
			Password:     as.passwordField.Value(),
			Token:        as.tokenField.Value(),
			KeyName:      as.keyNameField.Value(),
			KeyIn:        keyIn,
			KeyValue:     as.keyValueField.Value(),
			GrantType:    grantType,
			ClientID:     as.clientIDField.Value(),
			ClientSecret: as.clientSecretField.Value(),
			AuthURL:      as.authURLField.Value(),
			TokenURL:     as.tokenURLField.Value(),
			Scopes:       as.scopesField.Value(),
		})
	}
	return schemes
}

func (a auth) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width

	case tabs.SetActiveTabMsg:
		a.SetActive(msg.Active)
		return &a, nil

	case tea.KeyMsg:
		// A chord works even when tab is not active
		if a.focusPos < 0 {
			switch {
			case msg.String() == "A":
				a.schemes = append(a.schemes, newAuthScheme())
				if a.focusPos < 0 {
					a.focusPos = 0
				}
				return &a, nil
			}
		}

		if !a.active {
			return &a, nil
		}

		total := a.totalFields()

		if a.focusPos >= 0 && total > 0 {
			switch {
			case key.Matches(msg, config.DefaultKeyMap.Delete):
				si, fi := a.resolveField(a.focusPos)
				if fi == 0 {
					a.schemes = append(a.schemes[:si], a.schemes[si+1:]...)
					if len(a.schemes) == 0 {
						a.focusPos = -1
					} else {
						a.focusPos = max(0, min(a.focusPos, a.totalFields()-1))
					}
					return &a, nil
				}

			case msg.Type == tea.KeyCtrlT:
				si, _ := a.resolveField(a.focusPos)
				newEnabled := !a.schemes[si].enabled
				for j := range a.schemes {
					a.schemes[j].enabled = j == si && newEnabled
				}
				return &a, nil
			}
		}

		// Navigation
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Next):
			if total > 0 {
				a.focusPos = (a.focusPos + 1) % total
			}

		case key.Matches(msg, config.DefaultKeyMap.Prev):
			if total > 0 {
				a.focusPos = (a.focusPos - 1 + total) % total
			}

		default:
			if a.focusPos < 0 || total == 0 {
				return &a, nil
			}

			si, fi := a.resolveField(a.focusPos)
			scheme, cmd := a.schemes[si].updateField(fi, msg)
			a.schemes[si] = scheme
			return &a, cmd
		}
	}

	return &a, nil
}
