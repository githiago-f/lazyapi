package editor

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/model"
)

func authKey(a *auth, s string) tea.Cmd {
	var k tea.KeyPressMsg
	switch s {
	case "tab":
		k = tea.KeyPressMsg{Code: tea.KeyTab}
	case "enter":
		k = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "down":
		k = tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+r":
		k = tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
	case "ctrl+l":
		k = tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl}
	default:
		k = tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
	m, cmd := a.Update(k)
	*a = *m.(*auth)
	return cmd
}

func TestAuthTab_ReuseSpecSchemeAndOIDC(t *testing.T) {
	a := AuthorizeTab()
	a.SetSpec("api.yml", []model.AuthScheme{
		{Type: model.AuthOpenIDConnect, SchemeName: "keycloak", Issuer: "https://sso/realms/acme", GrantType: "authorizationCode", Scopes: "openid"},
		{Type: model.AuthAPIKey, SchemeName: "key", KeyName: "X-Key", KeyIn: "header"},
	})
	m, _ := a.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	a = m.(*auth)
	a.SetActive(true)

	authKey(a, "ctrl+r") // new scheme, focus on the Scheme select
	// enter opens the Scheme select, "k" jumps to "keycloak", enter picks it.
	for _, k := range []string{"enter", "k", "enter"} {
		authKey(a, k)
	}
	got := a.Value()
	if len(got) != 1 || got[0].SchemeName != "keycloak" || got[0].Type != model.AuthOpenIDConnect || got[0].Issuer != "https://sso/realms/acme" {
		t.Fatalf("reused scheme: %+v", got)
	}
	view := ansi.Strip(a.View().Content)
	for _, want := range []string{"Issuer URL", "Grant", "Redirect URI", "Login"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}

	// ctrl+l asks the TUI to log in to the focused scheme.
	cmd := authKey(a, "ctrl+l")
	if cmd == nil {
		t.Fatal("ctrl+l: no command")
	}
	lm, ok := cmd().(LoginMsg)
	if !ok || lm.Spec != "api.yml" || lm.Scheme.SchemeName != "keycloak" {
		t.Errorf("login msg: %#v", lm)
	}

	// Changing the grant changes the rows: password shows the user fields.
	a.SetValue([]model.AuthScheme{{Type: model.AuthOAuth2, SchemeName: "o", GrantType: "password", TokenURL: "https://t"}})
	view = ansi.Strip(a.View().Content)
	if !strings.Contains(view, "Username") || strings.Contains(view, "Redirect URI") || strings.Contains(view, "Authorization URL") {
		t.Errorf("password grant rows:\n%s", view)
	}
	if v := a.Value()[0]; v.GrantType != "password" || v.TokenURL != "https://t" || v.SchemeName != "o" {
		t.Errorf("roundtrip: %+v", v)
	}
}

func TestAuthTab_ClickFocusesAndOpensSelect(t *testing.T) {
	components.Z.Clear()
	a := AuthorizeTab()
	m, _ := a.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	a = m.(*auth)
	a.SetActive(true)
	a.SetValue([]model.AuthScheme{{Type: model.AuthBearer, SchemeName: "b", Token: "t"}})
	components.Z.Scan(a.View().Content)

	center := func(z components.Zone) tea.MouseClickMsg {
		return tea.MouseClickMsg{X: z.StartX + 1, Y: (z.StartY + z.EndY) / 2, Button: tea.MouseLeft}
	}
	click := func(z components.Zone) {
		t.Helper()
		if z.IsZero() {
			t.Fatal("zone not drawn")
		}
		m, _ := a.Update(center(z))
		a = m.(*auth)
		components.Z.Scan(a.View().Content)
	}
	// Rows: Scheme, Name, Type, Token. Click the Type select: it opens.
	click(a.schemes[0].typeSelector.Zone())
	if a.focusPos != 2 || !a.schemes[0].typeSelector.Open || !a.Completing() {
		t.Fatalf("focus=%d open=%v", a.focusPos, a.schemes[0].typeSelector.Open)
	}
	// Pick "API Key" in the open list.
	click(components.Z.Get("select:" + a.schemes[0].typeSelector.ID + ":2"))
	if v := a.Value()[0]; v.Type != model.AuthAPIKey || a.schemes[0].typeSelector.Open {
		t.Fatalf("picked %v", v.Type)
	}
	// Clicking the Value field focuses it (rows: Scheme, Name, Type, Key name, In, Value).
	click(components.Z.Get(a.schemes[0].rowZone(5)))
	if a.focusPos != 5 {
		t.Errorf("focus=%d", a.focusPos)
	}
}
