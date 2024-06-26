package normal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nobe4/gh-not/internal/actors"
	"github.com/nobe4/gh-not/internal/config"
	"github.com/nobe4/gh-not/internal/notifications"
	"github.com/nobe4/gh-not/internal/views"
	"github.com/nobe4/gh-not/internal/views/command"
	"github.com/nobe4/gh-not/internal/views/filter"
)

type Keymap struct {
	up       key.Binding
	down     key.Binding
	next     key.Binding
	previous key.Binding
	toggle   key.Binding
	all      key.Binding
	none     key.Binding
	filter   key.Binding
	command  key.Binding
	open     key.Binding
	help     key.Binding
	quit     key.Binding
}

func (k Keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.help}
}

func (k Keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.up, k.down, k.next, k.previous, k.toggle, k.all, k.none, k.open},
		{k.filter, k.command, k.help, k.quit},
	}
}

type SelectMsg struct {
	id       int
	selected bool
}

type OpenMessage struct {
	notification *notifications.Notification
}

type NoopMsg struct{}

type Model struct {
	Mode views.Mode
	Keys Keymap
	help help.Model

	notificationsRefresh func() notifications.Notifications
	choices              notifications.Notifications
	renderCache          []string

	cursor         int
	visibleChoices []int
	paginator      paginator.Model

	selected map[int]bool

	filter  tea.Model
	command tea.Model

	height int

	actors actors.ActorsMap

	result string
}

func New(actors actors.ActorsMap, notificationRefresh func() notifications.Notifications, keymap config.Keymap, view config.View) Model {
	model := Model{
		Mode: views.NormalMode,
		Keys: Keymap{
			up:       keymap.Binding("normal", "cursor up"),
			down:     keymap.Binding("normal", "cursor down"),
			next:     keymap.Binding("normal", "next page"),
			previous: keymap.Binding("normal", "previous page"),
			toggle:   keymap.Binding("normal", "toggle selected"),
			all:      keymap.Binding("normal", "select all"),
			none:     keymap.Binding("normal", "select none"),
			open:     keymap.Binding("normal", "open in browser"),
			filter:   keymap.Binding("normal", "filter mode"),
			command:  keymap.Binding("normal", "command mode"),
			help:     keymap.Binding("normal", "toggle help"),
			quit:     keymap.Binding("normal", "quit"),
		},
		help:                 help.New(),
		cursor:               0,
		selected:             map[int]bool{},
		notificationsRefresh: notificationRefresh,
		height:               view.Height,
		actors:               actors,
		paginator:            paginator.New(),
	}

	model.command = command.New(actors, model.SelectedNotificationsFunc, keymap)
	model.filter = filter.New(model.VisibleLinesFunc, keymap)

	// handling it in normal mode to display the help
	model.paginator.KeyMap = paginator.KeyMap{}
	model.paginator.PerPage = model.height

	return model
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.refreshNotifications(), m.filter.Init())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case NoopMsg:

	case filter.FilterMsg:
		m.visibleChoices = msg.IntSlice()
		m.paginator.Page = 0
		m.paginator.SetTotalPages(len(m.visibleChoices))

	case views.ResultMsg:
		m.result = msg.String()
		return m, tea.Batch(m.refreshNotifications(), views.ChangeMode(views.ResultMode))

	case SelectMsg:
		m.selected[msg.id] = msg.selected

	case OpenMessage:
		m.actors["open"].Run(msg.notification, nil)

	case views.ChangeModeMsg:
		m.Mode = msg.Mode

	case tea.KeyMsg:
		switch m.Mode {
		case views.NormalMode:
			switch {
			case key.Matches(msg, m.Keys.up):
				if m.cursor > 0 {
					m.cursor--
				}

			case key.Matches(msg, m.Keys.down):
				if m.cursor < len(m.visibleChoices)-1 {
					m.cursor++
				}

			case key.Matches(msg, m.Keys.next):
				m.paginator.NextPage()

			case key.Matches(msg, m.Keys.previous):
				m.paginator.PrevPage()

			case key.Matches(msg, m.Keys.toggle):
				return m, m.toggleSelect()

			case key.Matches(msg, m.Keys.all):
				return m, m.selectAll(true)

			case key.Matches(msg, m.Keys.none):
				return m, m.selectAll(false)

			case key.Matches(msg, m.Keys.open):
				return m, m.openCurrent()

			case key.Matches(msg, m.Keys.filter):
				return m, views.ChangeMode(views.SearchMode)

			case key.Matches(msg, m.Keys.command):
				return m, views.ChangeMode(views.CommandMode)

			case key.Matches(msg, m.Keys.help):
				return m, views.ChangeMode(views.HelpMode)

			case key.Matches(msg, m.Keys.quit):
				return m, tea.Quit

			}

		case views.SearchMode:
			m.filter, cmd = m.filter.Update(msg)
			cmds = append(cmds, cmd)

		case views.CommandMode:
			m.command, cmd = m.command.Update(msg)
			cmds = append(cmds, cmd)

		case views.ResultMode:
			return m, views.ChangeMode(views.NormalMode)

		case views.HelpMode:
			return m, views.ChangeMode(views.NormalMode)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.Mode == views.ResultMode {
		return m.result
	}

	if m.Mode == views.HelpMode {
		m.help.ShowAll = true
		return m.help.View(m.Keys)
	} else {
		m.help.ShowAll = false
	}

	out := ""

	// padding to keep the help at the bottom
	visibleChoicesLen := len(m.visibleChoices)
	for i := visibleChoicesLen; i < m.height; i++ {
		out += "\n"
	}

	start, end := m.paginator.GetSliceBounds(visibleChoicesLen)
	for i, id := range m.visibleChoices[start:end] {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		checked := " "
		if v, ok := m.selected[id]; ok && v {
			checked = "x"
		}

		out += fmt.Sprintf("%s%s%s\n", checked, cursor, m.renderCache[id])
	}

	if m.paginator.TotalPages > 1 {
		out += m.paginator.View() + " "
	}

	switch m.Mode {
	case views.NormalMode:
		out += m.help.View(m.Keys)
	case views.SearchMode:
		out += m.filter.View()
	case views.CommandMode:
		out += m.command.View()
	}

	return out
}

func (m Model) SelectedNotificationsFunc(cb func(*notifications.Notification)) {
	for i, selected := range m.selected {
		if selected {
			cb(m.choices[i])
		}
	}
}

func (m Model) VisibleLinesFunc(cb func(string, int)) {
	for i, line := range m.renderCache {
		cb(line, i)
	}
}

func (m Model) toggleSelect() tea.Cmd {
	return func() tea.Msg {
		visibleLineId := m.visibleChoices[m.cursor]
		selected, ok := m.selected[visibleLineId]

		return SelectMsg{
			id:       visibleLineId,
			selected: !(selected && ok),
		}
	}
}
func (m Model) selectAll(selected bool) tea.Cmd {
	cmds := tea.BatchMsg{}

	for _, id := range m.visibleChoices {
		cmds = append(
			cmds,
			func() tea.Msg {
				return SelectMsg{id: id, selected: selected}
			},
		)
	}

	return tea.Batch(cmds...)
}

func (m Model) openCurrent() tea.Cmd {
	return func() tea.Msg {
		visibleLineId := m.visibleChoices[m.cursor]
		return OpenMessage{notification: m.choices[visibleLineId]}
	}
}

func (m *Model) refreshNotifications() {
	m.cursor = 0
	m.paginator.Page = 0
	m.selected = map[int]bool{}

	m.choices = m.notificationsRefresh()

	renderCache, err := m.choices.Table()
	if err != nil {
		// TODO: handle error
		panic(err)
	}

	m.renderCache = strings.Split(renderCache, "\n")
}
