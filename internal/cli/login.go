package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/oauth"
	"github.com/githiago-f/lazyapi/internal/project"
	"github.com/githiago-f/lazyapi/internal/store"
)

const loginUsage = `usage: lazyapi login <spec.yml> [scheme] [flags]
       lazyapi logout <spec.yml> [scheme]

Logs in to an OAuth2 / OpenID Connect security scheme and keeps the session
in .lazyapi/.local/tokens.json (git-ignored, 0600). run, send, flows and the
TUI then use the token and refresh it on their own.

The scheme comes from the spec (oauth2 / openIdConnect) with the
environment's auth: from .lazyapi/config.yml, e.g. for Keycloak:

  environments:
    dev:
      auth:
        keycloak:
          issuer: https://sso.example.com/realms/acme   # endpoints discovered
          clientId: lazyapi
          grantType: authorizationCode                  # or deviceCode, clientCredentials, password

Flags:
  --env-name name   environment from .lazyapi/config.yml
  --env file        extra env file ({{env.X}})
  --device          use the device code grant (no local browser needed)
  --no-browser      only print the login URL
  --status          list the stored sessions`

// loginTarget resolves the spec's OAuth schemes with the environment.
func loginTarget(specPath string, args []string, envName, envFile string) (model.AuthScheme, env.Context, *project.Resolved, error) {
	spec, err := loadSpec(specPath)
	if err != nil {
		return model.AuthScheme{}, env.Context{}, nil, err
	}
	selected, err := project.Resolve(specPath, envName, "", envFile)
	if err != nil {
		return model.AuthScheme{}, env.Context{}, nil, usageErr("%v", err)
	}
	envMap, err := env.NewStore(selected.EnvFiles...).Load()
	if err != nil {
		return model.AuthScheme{}, env.Context{}, nil, runtimeErr(fmt.Errorf("error loading env file: %w", err))
	}
	schemes := model.OAuthSchemes(store.SecuritySchemes(spec), selected.Auth)
	names := make([]string, len(schemes))
	for i, s := range schemes {
		names[i] = s.SchemeName
	}
	tctx := env.Context{Env: envMap, Vars: selected.Vars}
	switch {
	case len(schemes) == 0:
		return model.AuthScheme{}, tctx, &selected, usageErr("%s has no oauth2 or openIdConnect security scheme (and the environment's auth names no issuer)", specPath)
	case len(args) == 0 && len(schemes) == 1:
		return schemes[0], tctx, &selected, nil
	case len(args) == 0:
		return model.AuthScheme{}, tctx, &selected, usageErr("several schemes, name one: %s", strings.Join(names, ", "))
	}
	for _, s := range schemes {
		if s.SchemeName == args[0] {
			return s, tctx, &selected, nil
		}
	}
	return model.AuthScheme{}, tctx, &selected, usageErr("unknown scheme %q (oauth schemes: %s)", args[0], strings.Join(names, ", "))
}

// Login runs `lazyapi login`.
func Login(args []string) error {
	fs := newFlagSet("login")
	envName := fs.String("env-name", "", "environment from "+project.FileName)
	envFile := fs.String("env", "", "environment file")
	device := fs.Bool("device", false, "use the device code grant")
	noBrowser := fs.Bool("no-browser", false, "only print the login URL")
	status := fs.Bool("status", false, "list the stored sessions")
	pos, err := parseInterleaved(fs, args)
	if errors.Is(err, flag.ErrHelp) || len(pos) == 0 {
		fmt.Println(loginUsage)
		if len(pos) == 0 && err == nil {
			return usageErr("missing spec")
		}
		return nil
	}
	if err != nil {
		return usageErr("%v\n%s", err, loginUsage)
	}
	specPath := pos[0]
	tokens := oauth.StoreFor(specPath)
	if *status {
		printSessions(tokens)
		return nil
	}

	scheme, tctx, selected, err := loginTarget(specPath, pos[1:], *envName, *envFile)
	if err != nil {
		return err
	}
	cfg := model.OAuthConfig(scheme, tctx)
	if *device {
		cfg.Grant = oauth.GrantDeviceCode
	}
	client, err := selected.HTTP.Client()
	if err != nil {
		return usageErr("%v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	tok, err := oauth.Login(ctx, client, cfg, tokens, oauth.UI{
		NoBrowser: *noBrowser,
		Notice:    func(msg string) { fmt.Fprintln(os.Stderr, msg) },
	})
	if err != nil {
		return runtimeErr(err)
	}
	fmt.Printf("✓ logged in to %s%s (%s)\n", scheme.SchemeName, envSuffix(selected.Name), expiresIn(tok))
	return nil
}

// Logout runs `lazyapi logout`.
func Logout(args []string) error {
	fs := newFlagSet("logout")
	pos, err := parseInterleaved(fs, args)
	if err != nil || len(pos) == 0 || len(pos) > 2 {
		return usageErr("%s", loginUsage)
	}
	name := ""
	if len(pos) == 2 {
		name = pos[1]
	}
	n, err := oauth.Logout(name, oauth.StoreFor(pos[0]))
	if err != nil {
		return runtimeErr(err)
	}
	fmt.Printf("removed %d session(s)\n", n)
	return nil
}

func envSuffix(name string) string {
	if name == "" {
		return ""
	}
	return " [" + name + "]"
}

func expiresIn(t oauth.Token) string {
	if t.Expiry.IsZero() {
		return "no expiry"
	}
	d := time.Until(t.Expiry).Round(time.Second)
	if d <= 0 {
		if t.RefreshToken != "" {
			return "expired, will refresh"
		}
		return "expired"
	}
	s := "expires in " + d.String()
	if t.RefreshToken != "" {
		s += ", refreshable"
	}
	return s
}

func printSessions(s *oauth.Store) {
	list := s.List()
	if len(list) == 0 {
		fmt.Println("no sessions")
		return
	}
	for _, t := range list {
		where := t.Issuer
		if where == "" {
			where = "-"
		}
		fmt.Printf("%-20s %-50s %s\n", t.Name, where, expiresIn(t))
	}
}
