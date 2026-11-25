package app

import (
	"context"
	"fmt"
	"regexp"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/githiago-f/lazyapi/internal/app/pane/editor"
	"github.com/githiago-f/lazyapi/internal/app/pane/responses"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/oauth"
	"github.com/githiago-f/lazyapi/internal/store"
)

// OAuth logins run in a goroutine; what they tell the user (the URL to
// open, the device code) and their result come back through a channel.
type (
	// loginMsg starts a login (palette, the Auth tab's Login button, or a
	// send that found no session). Retry is resent once logged in.
	loginMsg struct {
		spec   string
		scheme model.AuthScheme // secrets applied
		retry  *responses.SendMsg
	}
	loginNoticeMsg struct {
		text string
		ch   <-chan tea.Msg
	}
	loginDoneMsg struct {
		name  string
		token oauth.Token
		err   error
		retry *responses.SendMsg
	}
	logoutMsg struct {
		spec string
		name string // "" = every session
	}
)

// authSecrets are the active environment's credentials.
func (t Tui) authSecrets() map[string]model.AuthSecrets {
	if t.activeEnv == nil {
		return nil
	}
	return t.activeEnv.Auth
}

// templateContext resolves {{env.X}} / {{var.X}} like a send does.
func (t Tui) templateContext() env.Context {
	envMap, _ := t.envStore.Load()
	var vars map[string]string
	if t.activeEnv != nil {
		vars = t.activeEnv.Vars
	}
	return env.Context{Env: envMap, Vars: vars}
}

// oauthSchemes lists the schemes of a spec one can log in to.
func (t Tui) oauthSchemes(spec string) []model.AuthScheme {
	doc, err := store.ParseSpec(spec)
	if err != nil {
		return nil
	}
	return model.OAuthSchemes(store.SecuritySchemes(doc), t.authSecrets())
}

// sessionStatus describes the stored session of a scheme ("" when none).
func (t Tui) sessionStatus(spec string, s model.AuthScheme) string {
	s = model.ApplySecrets([]model.AuthScheme{s}, t.authSecrets())[0]
	cfg := model.OAuthConfig(s, t.templateContext())
	if cfg.ClientID == "" {
		return config.T("editor.auth.session.no_client")
	}
	login := config.DefaultKeyMap.Login.Help().Key
	tok := oauth.Status(cfg, oauth.StoreFor(spec))
	switch {
	case tok == nil && oauth.Interactive(cfg.Grant):
		return config.T("editor.auth.session.not_logged_in", "login", login)
	case tok == nil:
		return config.T("editor.auth.session.on_send")
	case tok.Valid():
		return config.T("editor.auth.session.logged_in", "expires", until(tok.Expiry))
	case tok.RefreshToken != "":
		return config.T("editor.auth.session.refreshes")
	}
	return config.T("editor.auth.session.expired", "login", login)
}

func until(t time.Time) string {
	if t.IsZero() {
		return "no expiry"
	}
	d := time.Until(t).Round(time.Second)
	if d >= time.Hour {
		d = d.Round(time.Minute)
	}
	return "expires in " + d.String()
}

var linkPattern = regexp.MustCompile(`https?://\S+`)

// openBrowser opens login pages (tests replace it).
var openBrowser = oauth.OpenBrowser

// startLogin runs oauth.Login for a scheme; a running login is cancelled.
func (t Tui) startLogin(msg loginMsg) (Tui, tea.Cmd) {
	if t.cancelLogin != nil {
		t.cancelLogin()
	}
	s := model.ApplySecrets([]model.AuthScheme{msg.scheme}, t.authSecrets())[0]
	cfg := model.OAuthConfig(s, t.templateContext())
	var opts model.HTTPOptions
	if t.activeEnv != nil {
		opts = t.activeEnv.HTTP
	}
	client, err := opts.Client()
	if err != nil {
		return t, notify("Environment http options: "+err.Error(), components.Error)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.cancelLogin = cancel
	ch := make(chan tea.Msg, 8)
	tokens := oauth.StoreFor(msg.spec)
	go func() {
		defer close(ch)
		tok, err := oauth.Login(ctx, client, cfg, tokens, oauth.UI{
			Open: openBrowser,
			Notice: func(text string) {
				// The URL (or the device code) is also on the clipboard:
				// notifications are short and browsers may not open.
				if link := linkPattern.FindString(text); link != "" {
					_ = clipboard.WriteAll(link)
				}
				ch <- loginNoticeMsg{text: text}
			},
		})
		ch <- loginDoneMsg{name: s.SchemeName, token: tok, err: err, retry: msg.retry}
	}()
	return t, tea.Batch(
		notify("🔑 Logging in to "+s.SchemeName+"…", components.Info),
		waitLogin(ch),
	)
}

func waitLogin(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		m, ok := <-ch
		if !ok {
			return nil
		}
		if n, ok := m.(loginNoticeMsg); ok {
			n.ch = ch
			return n
		}
		return m
	}
}

// updateLogin handles the login messages (ok is false for other messages).
func (t Tui) updateLogin(msg tea.Msg) (Tui, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case loginMsg:
		t, cmd := t.startLogin(msg)
		return t, cmd, true

	case editor.LoginMsg:
		t, cmd := t.startLogin(loginMsg{spec: msg.Spec, scheme: msg.Scheme})
		return t, cmd, true

	case loginNoticeMsg:
		note := func() tea.Msg {
			return components.ShowNotificationMsg{Message: msg.text + " (copied)", Type: components.Info, Timeout: time.Minute}
		}
		return t, tea.Batch(note, waitLogin(msg.ch)), true

	case loginDoneMsg:
		t.cancelLogin = nil
		if msg.err != nil {
			return t, notify("✗ Login failed: "+msg.err.Error(), components.Error), true
		}
		done := notify(fmt.Sprintf("✓ Logged in to %s · %s", msg.name, until(msg.token.Expiry)), components.Success)
		if msg.retry != nil {
			retry := *msg.retry
			retry.AfterLogin = true
			t, cmd := t.startSend(retry)
			return t, tea.Batch(done, cmd), true
		}
		return t, done, true

	case logoutMsg:
		n, err := oauth.Logout(msg.name, oauth.StoreFor(msg.spec))
		if err != nil {
			return t, notify("✗ Logout: "+err.Error(), components.Error), true
		}
		return t, notify(fmt.Sprintf("Removed %d session(s)", n), components.Success), true
	}
	return t, nil, false
}
