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
	Description components.Text
}

func DocumentationTab() *documentation {
	return &documentation{
		Summary:     components.InitField("Summary", ""),
		Description: components.NewTextarea("Longer request description", ""),
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
	d.Description.Style = d.Description.Style.UnsetBorderForeground()

	if d.active {
		switch docField(d.fieldCursor) {
		case summary:
			d.Summary.Style = d.Summary.Style.BorderForeground(activeColor)
		case description:
			d.Description.Style = d.Description.Style.BorderForeground(activeColor)
		}
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Top,
		d.Summary.View().Content,
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

func (d documentation) Init() tea.Cmd {
	return nil
}

func (d documentation) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Next) && d.active:
			d.fieldCursor = inmath.Circle(d.fieldCursor+1, 0, int(description))
		case key.Matches(msg, config.DefaultKeyMap.Prev) && d.active:
			d.fieldCursor = inmath.Circle(d.fieldCursor-1, 0, int(description))
		}

	case tea.WindowSizeMsg:
		d.Summary.Style = d.Summary.Style.Width(msg.Width)
		d.Summary.TextInput.SetWidth(max(0, msg.Width-2))
		descHeight := max(1, msg.Height-8)
		d.Description.Style = d.Description.Style.Width(msg.Width).Height(descHeight)
		d.Description.TextArea.SetWidth(max(0, msg.Width-2))
		d.Description.TextArea.SetHeight(descHeight)

	case tabs.SetActiveTabMsg:
		d.active = msg.Active
		return &d, nil
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
			model, cmd = d.Description.Update(msg)
			d.Description, _ = model.(components.Text)
		}
	}

	return &d, cmd
}
