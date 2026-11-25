package editor

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/inmath"
	"github.com/githiago-f/lazyapi/internal/model"
)

type docField int

const (
	summary docField = iota
	description
)

type documentation struct {
	Style       lipgloss.Style
	fieldCursor int
	active      bool

	Summary     components.Field
	Description components.CodeEditor
}

func DocumentationTab() *documentation {
	return &documentation{
		Summary:     components.InitField(config.T("editor.docs.summary"), ""),
		Description: components.NewCodeEditor(components.LangMarkdown, config.T("editor.docs.description"), ""),
	}
}

func (d documentation) SetValue(val model.About) documentation {
	d.Summary.SetValue(val.Summary)
	d.Description.SetValue(val.Description)

	return d
}

func (d *documentation) Value() model.About {
	return model.About{
		Summary:     d.Summary.Value(),
		Description: d.Description.Value(),
	}
}

func (d *documentation) SetActive(b bool) {
	d.active = b
}

func (d *documentation) IsActive() bool {
	return d.active
}

func (d documentation) View() tea.View {
	activeColor := config.DefaultConfig.PrimaryColor()

	d.Summary.Style = d.Summary.Style.UnsetBorderForeground()
	if d.active && docField(d.fieldCursor) == summary {
		d.Summary.Style = d.Summary.Style.BorderForeground(activeColor)
	}
	if d.active && docField(d.fieldCursor) == description {
		d.Description.Focus()
	} else {
		d.Description.Blur()
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Top,
		components.Z.Mark(summaryZone, d.Summary.View().Content),
		d.Description.View().Content,
	))
}

func (d documentation) HelpBindings() []key.Binding {
	return []key.Binding{
		config.DefaultKeyMap.Next,
		config.DefaultKeyMap.Prev,
		config.DefaultKeyMap.Back,
	}
}

const summaryZone = "editor:docs-summary"

func (d documentation) Init() tea.Cmd {
	return nil
}

func (d documentation) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if d.Completing() && menuKey(msg) {
			break // the open menu takes tab/esc/arrows
		}
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Next) && d.active:
			d.fieldCursor = inmath.Circle(d.fieldCursor+1, 0, int(description))
			return &d, nil
		case key.Matches(msg, config.DefaultKeyMap.Prev) && d.active:
			d.fieldCursor = inmath.Circle(d.fieldCursor-1, 0, int(description))
			return &d, nil
		}

	case tea.WindowSizeMsg:
		d.Summary.Style = d.Summary.Style.Width(msg.Width)
		d.Summary.TextInput.SetWidth(max(0, msg.Width-2))
		d.Description.SetSize(msg.Width, max(4, msg.Height-6))

	case tabs.SetActiveTabMsg:
		d.active = msg.Active
		return &d, nil

	case tea.MouseClickMsg:
		if z := components.Z.Get(summaryZone); z.InBounds(msg) {
			d.fieldCursor = int(summary)
			x, _ := z.Pos(msg)
			d.Summary.ClickAt(x)
			return &d, nil
		}
		if d.Description.Zone().InBounds(msg) {
			d.fieldCursor = int(description)
		}
	}

	var (
		model tea.Model
		cmd   tea.Cmd
	)
	if d.active {
		switch docField(d.fieldCursor) {
		case summary:
			model, cmd = d.Summary.Update(msg)
			d.Summary, _ = model.(components.Field)
		case description:
			d.Description.Focus()
			d.Description, cmd = d.Description.Update(msg)
		}
	}

	return &d, cmd
}

// Completing reports whether a field has its completion menu open.
func (d documentation) Completing() bool {
	if !d.active {
		return false
	}
	if docField(d.fieldCursor) == summary {
		return d.Summary.Completing()
	}
	return d.Description.Completing()
}

// HasSelection reports whether the description has selected text.
func (d documentation) HasSelection() bool {
	return d.active && docField(d.fieldCursor) == description && d.Description.HasSelection()
}
