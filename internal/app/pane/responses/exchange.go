package responses

import (
	"context"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/runner"
	"github.com/githiago-f/lazyapi/internal/script"
	"github.com/githiago-f/lazyapi/internal/store"
)

// Source identifies which view asked for a request.
type Source int

const (
	FromEditor Source = iota
	FromPreview
)

// SendMsg asks the TUI to send a request. The TUI owns cancellation.
type SendMsg struct {
	Source  Source
	Request model.Request
	// Script is the test script to run; nil means "the operation's saved
	// script" (the preview), a pointer to "" means none.
	Script *string
	// Auth and Vars come from the active named environment.
	Auth map[string]model.AuthSecrets
	Vars map[string]string
	HTTP model.HTTPOptions
}

// ResultMsg carries a finished exchange back to the view that sent it.
type ResultMsg struct {
	Source Source
	Result runner.OpResult
	// Request is what was sent (templates unresolved), for the history.
	Request model.Request
}

// Exchange returns a command that sends req through the runner, so the TUI
// shows the same status, contract and script checks as `lazyapi run`.
func Exchange(ctx context.Context, msg SendMsg) tea.Cmd {
	return func() tea.Msg {
		return ResultMsg{Source: msg.Source, Result: exchange(ctx, msg), Request: msg.Request}
	}
}

func exchange(ctx context.Context, msg SendMsg) runner.OpResult {
	req := msg.Request
	req.Auth = model.ApplySecrets(req.Auth, msg.Auth)
	client, err := msg.HTTP.Client()
	if err != nil {
		return runner.OpResult{Method: req.Method.Label(), Path: req.URI, Outcome: runner.Errored, Reason: err.Error()}
	}
	req.HTTPClient = client
	in := runner.ExchangeInput{Request: req, Ref: req.OpenAPIRef, Auth: msg.Auth}
	if req.Timeout == 0 {
		in.Request.Timeout = model.DefaultTimeout
	}
	vars := map[string]string{}
	for k, v := range msg.Vars {
		vars[k] = v
	}
	for k, v := range in.Request.Vars {
		vars[k] = v
	}
	in.Request.Vars = vars

	var modules *script.ModuleLoader
	if req.FileName != "" {
		if spec, err := store.ParseSpec(req.FileName); err == nil {
			in.Spec, in.SpecPath = spec, req.FileName
			specDir := filepath.Dir(req.FileName)
			modules = script.NewModuleLoader(specDir)
			in.ScriptOptions = script.Options{
				Filename: "tests.js", ResolveDir: filepath.Join(specDir, store.TestsDir), Root: specDir,
			}
			if req.OpenAPIRef != nil {
				if op := store.FindOperation(spec, *req.OpenAPIRef); op != nil {
					if o, err := script.OperationOptions(req.FileName, op, *req.OpenAPIRef); err == nil {
						in.ScriptOptions = o
					}
					if msg.Script == nil {
						in.Script, _ = store.LoadOperationTests(req.FileName, op)
					}
				}
			}
			in.ScriptOptions.Modules = modules
		}
	}
	if msg.Script != nil {
		in.Script = *msg.Script
	}

	res := runner.Exchange(ctx, in)
	if err := modules.SaveLock(); err != nil {
		res.Checks = append(res.Checks, runner.Check{Kind: runner.CheckScript, Name: "lockfile saved", Message: err.Error()})
	}
	return res
}
