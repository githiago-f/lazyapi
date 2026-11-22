package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"

	tea "charm.land/bubbletea/v2"
	"github.com/githiago-f/lazyapi/internal/app/pane/palette"
	"github.com/githiago-f/lazyapi/internal/app/pane/requests"
	"github.com/githiago-f/lazyapi/internal/app/pane/runview"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/runner"
	"github.com/githiago-f/lazyapi/internal/script"
	"github.com/githiago-f/lazyapi/internal/store"
)

// Palette actions (the Msg of palette.Command).
type (
	// runTestsMsg starts the runner view.
	runTestsMsg struct {
		spec      string
		tag       string   // "" = every tag
		methods   []string // nil = every method (needs confirmation)
		flows     []string // flows to run after the operations
		onlyFlows bool
		title     string
	}
	useEnvMsg     struct{ name string } // named environment ("" = none)
	useEnvFileMsg struct{ path string } // env file when there is no config
	copyCurlMsg   struct{}
	showRunnerMsg struct{}
	toggleHelpMsg struct{}
)

// specs lists the loaded specs (the default one first).
func (t Tui) specs() []string {
	var out []string
	if t.defaultFile != "" {
		out = append(out, t.defaultFile)
	}
	for _, p := range t.openAPIFiles {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// paletteCommands lists what the palette offers in the current state.
func (t Tui) paletteCommands() []palette.Command {
	var cmds []palette.Command
	add := func(group, title, hint string, msg tea.Msg) {
		cmds = append(cmds, palette.Command{Group: group, Title: title, Hint: hint, Msg: msg})
	}

	if config.DefaultConfig.Active == config.RequestEditor {
		add("Request", "Copy as cURL", "alt+c", copyCurlMsg{})
	}
	if len(t.runner.Results()) > 0 || t.runner.Running() {
		add("Run", "Show last run results", "", showRunnerMsg{})
	}

	specs := t.specs()
	multi := len(specs) > 1
	for _, spec := range specs {
		suffix := ""
		if multi {
			suffix = " · " + filepath.Base(spec)
		}
		add("Run", "Read-only operations"+suffix, "GET/HEAD/OPTIONS", runTestsMsg{spec: spec, methods: runner.SafeMethods, title: filepath.Base(spec)})
		add("Run", "All operations (writes included)"+suffix, "asks first", runTestsMsg{spec: spec, title: filepath.Base(spec) + " · all methods"})

		tags := map[string]bool{}
		for _, it := range t.requestList.Items() {
			if it.FileName == spec {
				for _, tag := range it.Tags {
					tags[tag] = true
				}
			}
		}
		names := make([]string, 0, len(tags))
		for tag := range tags {
			names = append(names, tag)
		}
		sort.Strings(names)
		for _, tag := range names {
			add("Run", "Tag "+tag+suffix, "read-only", runTestsMsg{spec: spec, tag: tag, methods: runner.SafeMethods, title: filepath.Base(spec) + " · tag " + tag})
		}

		flows, _ := runner.FindFlows(spec)
		if len(flows) > 0 {
			add("Flow", "All flows"+suffix, plural(len(flows), "flow"), runTestsMsg{spec: spec, flows: flows, onlyFlows: true, title: filepath.Base(spec) + " · flows"})
		}
		for _, f := range flows {
			rel, _ := filepath.Rel(filepath.Dir(spec), f)
			add("Flow", rel+suffix, "", runTestsMsg{spec: spec, flows: []string{f}, onlyFlows: true, title: rel})
		}
	}

	for _, it := range t.requestList.Items() {
		title := it.Method.Label() + " " + it.URI
		hint := it.About.Summary
		if it.DraftPath != "" {
			add("Open", title, "draft", requests.OpenRequestViewMsg{FileName: it.FileName, DraftPath: it.DraftPath})
			continue
		}
		if it.OpenAPIRef != nil {
			ref := *it.OpenAPIRef
			add("Open", title, hint, requests.OpenRequestViewMsg{FileName: it.FileName, OpenAPIRef: &ref})
		}
	}

	if t.project != nil {
		for _, name := range t.project.Names() {
			hint := ""
			if t.activeEnv != nil && t.activeEnv.Name == name {
				hint = "active"
			}
			add("Environment", name, hint, useEnvMsg{name: name})
		}
		add("Environment", "none", "", useEnvMsg{})
	} else {
		for _, p := range env.Discover(".") {
			hint := ""
			if p == t.envStore.Path() {
				hint = "active"
			}
			add("Environment", env.Label(p), hint, useEnvFileMsg{path: p})
		}
	}

	add("Request", "New request", "n", requests.CreateNewRequestMsg{})
	add("App", "Keyboard shortcuts", "?", toggleHelpMsg{})
	add("App", "Quit", "ctrl+c", tea.QuitMsg{})
	return cmds
}

// startRun prepares a run for the runner view. Runs that include writes
// wait for a y/N confirmation.
func (t Tui) startRun(msg runTestsMsg) (Tui, tea.Cmd) {
	spec, err := store.ParseSpec(msg.spec)
	if err != nil {
		return t, notify("Cannot load spec: "+err.Error(), components.Error)
	}
	envMap, _ := t.envStore.Load()
	opts := runner.Options{
		Env:            envMap,
		Timeout:        model.DefaultTimeout,
		Modules:        script.NewModuleLoader(filepath.Dir(msg.spec)),
		Flows:          msg.flows,
		SkipOperations: msg.onlyFlows,
		Filter:         runner.Filter{Methods: msg.methods},
	}
	if msg.tag != "" {
		opts.Filter.Tags = []string{msg.tag}
	}
	if e := t.activeEnv; e != nil {
		opts.EnvName, opts.Vars, opts.Auth = e.Name, e.Vars, e.Auth
		opts.Server = e.Server
		client, err := e.HTTP.Client()
		if err != nil {
			return t, notify("Environment http options: "+err.Error(), components.Error)
		}
		opts.HTTPClient = client
	}
	run := runview.Run{Title: msg.title, Spec: spec, SpecPath: msg.spec, Options: opts}
	// Writes (every method, or flows: scripts send whatever they want) ask first.
	if (msg.methods == nil && !msg.onlyFlows) || len(msg.flows) > 0 {
		t.confirmRun = &run
		return t, nil
	}
	return t.beginRun(run)
}

func (t Tui) beginRun(run runview.Run) (Tui, tea.Cmd) {
	t.runnerFrom = config.DefaultConfig.Active
	if t.runnerFrom == config.Runner {
		t.runnerFrom = config.RequestList
	}
	config.DefaultConfig.Active = config.Runner
	cmd := t.runner.Start(run)
	return t, cmd
}

// confirmRunText is the confirmation shown before a run with writes.
func (t Tui) confirmRunText() string {
	r := t.confirmRun
	question := "Run every operation, including POST/PUT/PATCH/DELETE?"
	what := plural(runner.PlannedCount(r.Spec, r.SpecPath, r.Options), "operation")
	if r.Options.SkipOperations {
		question = "Run flows? They may create, change or delete data."
		what = plural(len(r.Options.Flows), "flow")
	}
	return fmt.Sprintf("%s\n\n%s\n%s · env: %s\n\n%s",
		confirmTitle.Render(question),
		r.Title, what, t.envName(),
		confirmKey.Render("y")+" run   "+confirmKey.Render("any other key")+" cancel")
}

// beginShowRunner shows the runner view with the last results.
func (t Tui) beginShowRunner() (Tui, tea.Cmd) {
	if config.DefaultConfig.Active != config.Runner {
		t.runnerFrom = config.DefaultConfig.Active
	}
	config.DefaultConfig.Active = config.Runner
	return t, nil
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
