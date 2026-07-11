package components

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/githiago-f/lazyapi/internal/config"
)

type NotificationType int

const (
	Info NotificationType = iota
	Success
	Error
)

type notification struct {
	ID        string
	Message   string
	Type      NotificationType
	Timeout   time.Duration
	createdAt time.Time
}

type Notifications struct {
	notes   []notification
	limit   int
	width   int
	zonePfx string
	nextID  int
}

type ShowNotificationMsg struct {
	Message string
	Type    NotificationType
	Timeout time.Duration
}

type DismissNotificationMsg struct {
	ID string
}

type NotificationTickMsg struct {
	ID string
}

func NewNotifications() Notifications {
	return Notifications{
		limit:   5,
		zonePfx: zone.NewPrefix(),
	}
}

func (n Notifications) Init() tea.Cmd {
	return nil
}

func (n Notifications) Visible() bool {
	return len(n.notes) > 0
}

func (n Notifications) Update(msg tea.Msg) (Notifications, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowNotificationMsg:
		timeout := msg.Timeout
		if timeout <= 0 {
			timeout = 3 * time.Second
		}
		note := notification{
			ID:        fmt.Sprintf("%s-%d", n.zonePfx, n.nextID),
			Message:   msg.Message,
			Type:      msg.Type,
			Timeout:   timeout,
			createdAt: time.Now(),
		}
		n.nextID++
		n.notes = append([]notification{note}, n.notes...)
		if len(n.notes) > n.limit {
			n.notes = n.notes[:n.limit]
		}
		return n, tea.Tick(timeout, func(t time.Time) tea.Msg {
			return NotificationTickMsg{ID: note.ID}
		})

	case DismissNotificationMsg:
		for i, note := range n.notes {
			if note.ID == msg.ID {
				n.notes = append(n.notes[:i], n.notes[i+1:]...)
				break
			}
		}

	case NotificationTickMsg:
		for i, note := range n.notes {
			if note.ID == msg.ID {
				n.notes = append(n.notes[:i], n.notes[i+1:]...)
				break
			}
		}

	case tea.MouseMsg:
		if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
			break
		}
		for _, note := range n.notes {
			if zone.Get(note.ID).InBounds(msg) {
				for i, n2 := range n.notes {
					if n2.ID == note.ID {
						n.notes = append(n.notes[:i], n.notes[i+1:]...)
						break
					}
				}
				break
			}
		}
	}
	return n, nil
}

func (n Notifications) View() string {
	if len(n.notes) == 0 {
		return ""
	}

	var rendered []string
	for _, note := range n.notes {
		var color string
		switch note.Type {
		case Success:
			color = config.Green
		case Error:
			color = config.Red
		default:
			color = config.Blue
		}

		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(color)).
			Background(lipgloss.Color(config.Surface0)).
			Foreground(lipgloss.Color(color)).
			Padding(0, 2).
			Render(note.Message)

		rendered = append(rendered, zone.Mark(note.ID, box))
	}

	return lipgloss.JoinVertical(lipgloss.Top, rendered...)
}
