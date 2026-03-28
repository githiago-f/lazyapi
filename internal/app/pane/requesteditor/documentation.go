package requesteditor

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/components/tabs"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/inmath"
	"github.com/githiago-f/lazyapi/internal/model"
)

type docField int

const (
	title docField = iota
	summary
	description
)

type documentation struct {
	Style       lipgloss.Style
	fieldCursor int
	active      bool

	Title       components.Field
	Summary     components.Field
	Description components.Text
}

func DocumentationTab() *documentation {
	return &documentation{
		Title:       components.InitField("Title", ""),
		Summary:     components.InitField("Summary", ""),
		Description: components.NewTextarea("Longer request description", ""),
	}
}

// SetValue implements [tabs.StatefulInput].
func (d *documentation) SetValue(val model.About) {
	d.Title.SetValue(val.Title)
	d.Summary.SetValue(val.Summary)
	d.Description.SetValue(val.Description)
}

// Value implements [tabs.StatefulInput].
func (d *documentation) Value() model.About {
	return model.About{
		Title:       d.Title.Value(),
		Summary:     d.Summary.Value(),
		Description: d.Description.Value(),
	}
}

func (d documentation) SetActive(b bool) tabs.StatefulInputBase {
	d.active = b
	return d
}

func (d documentation) View() string {
	activeColor := config.DefaultConfig.PrimaryColor()

	if d.active {
		switch docField(d.fieldCursor) {
		case title:
			d.Title.Style = d.Title.Style.BorderForeground(activeColor)
		case summary:
			d.Summary.Style = d.Summary.Style.BorderForeground(activeColor)
		case description:
			d.Description.Style = d.Description.Style.BorderForeground(activeColor)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Top,
		d.Title.View(),
		d.Summary.View(),
		d.Description.View(),
	)
}

func (d documentation) Init() tea.Cmd {
	return nil
}

func (d documentation) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, config.DefaultKeyMap.Next) && d.active:
			d.fieldCursor = inmath.Cicle(d.fieldCursor+1, 0, int(description))
		case key.Matches(msg, config.DefaultKeyMap.Prev) && d.active:
			d.fieldCursor = inmath.Cicle(d.fieldCursor-1, 0, int(description))
		}

	case tea.WindowSizeMsg:
		d.Title.Style = d.Title.Style.Width(msg.Width)
		d.Summary.Style = d.Summary.Style.Width(msg.Width)
		d.Description.Style = d.Description.Style.Width(msg.Width)
	}

	var (
		model tea.Model
		cmd   tea.Cmd
	)
	switch docField(d.fieldCursor) {
	case title:
		model, cmd = d.Title.Update(msg)
		d.Title, _ = model.(components.Field)
	case summary:
		model, cmd = d.Summary.Update(msg)
		d.Summary, _ = model.(components.Field)
	case description:
		model, cmd = d.Description.Update(msg)
		d.Description, _ = model.(components.Text)
	}

	return d, cmd
}
