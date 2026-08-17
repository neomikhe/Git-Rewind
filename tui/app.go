package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/recipes"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

// PageSize is how many reflog entries the timeline loads at a time.
const PageSize = 500

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true)
	helpStyle     = lipgloss.NewStyle().Faint(true)
	labelStyle    = lipgloss.NewStyle().Faint(true)
	keyStyle      = lipgloss.NewStyle().Bold(true)
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
)

var errDirtyTree = errors.New("dirty working tree")

// Session is the repository state the TUI works on.
type Session struct {
	Git     *gitexec.Runner
	Events  []timeline.Event
	Limit   int
	Printer *i18n.Printer
}

// Run launches the timeline TUI and blocks until the user quits.
func Run(s Session) error {
	_, err := tea.NewProgram(newModel(s), tea.WithAltScreen()).Run()
	return err
}

type screen int

const (
	screenTimeline screen = iota
	screenDetail
	screenRescues
	screenConfirm
	screenResult
)

type rescue struct {
	recipe recipes.Recipe
	plan   *safety.Plan
}

type rescuesMsg struct {
	rescues []rescue
	dirty   bool
	err     error
}

type appliedMsg struct {
	result safety.Result
	err    error
}

type eventsMsg struct {
	events []timeline.Event
	limit  int
	err    error
}

type model struct {
	session Session
	screen  screen
	cursor  int
	choice  int
	height  int
	now     time.Time
	rescues []rescue
	dirty   bool
	loading bool
	help    bool
	applied *safety.Result
	err     error
}

func newModel(s Session) model {
	if s.Printer == nil {
		s.Printer = i18n.New(i18n.EN)
	}
	return model{session: s, now: time.Now()}
}

func (m model) hasMore() bool {
	return m.session.Limit > 0 && len(m.session.Events) >= m.session.Limit
}

func (m model) discardsUncommitted() bool {
	if m.choice >= len(m.rescues) {
		return false
	}
	return m.rescues[m.choice].plan.DiscardsChanges && m.dirty
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
	case eventsMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.session.Events, m.session.Limit = msg.events, msg.limit
		}
	case rescuesMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.rescues, m.dirty, m.choice = msg.rescues, msg.dirty, 0
			m.screen = screenRescues
		}
	case appliedMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.applied = &msg.result
			m.screen = screenResult
		}
	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "q" || key == "ctrl+c" {
		return m, tea.Quit
	}
	if key == "?" {
		m.help = !m.help
		return m, nil
	}
	if m.help {
		if key == "esc" {
			m.help = false
		}
		return m, nil
	}
	if m.loading {
		return m, nil
	}

	switch m.screen {
	case screenTimeline:
		return m.onTimelineKey(key)
	case screenDetail:
		return m.onDetailKey(key)
	case screenRescues:
		return m.onRescuesKey(key)
	case screenConfirm:
		return m.onConfirmKey(key)
	default:
		return m, nil
	}
}

func (m model) onTimelineKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.session.Events)-1 {
			m.cursor++
		}
	case "m":
		if m.hasMore() {
			m.err = nil
			m.loading = true
			return m, loadMoreCmd(m.session, m.session.Limit+PageSize)
		}
	case "enter", "right", "l":
		if len(m.session.Events) > 0 {
			m.err = nil
			m.screen = screenDetail
		}
	}
	return m, nil
}

func (m model) onDetailKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "left", "h", "backspace":
		m.screen = screenTimeline
	case "enter", "right", "l", "r":
		m.err = nil
		m.loading = true
		return m, detectCmd(m.session)
	}
	return m, nil
}

func (m model) onRescuesKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "left", "h", "backspace":
		m.screen = screenDetail
	case "up", "k":
		if m.choice > 0 {
			m.choice--
		}
	case "down", "j":
		if m.choice < len(m.rescues)-1 {
			m.choice++
		}
	case "enter", "right", "l":
		if len(m.rescues) > 0 {
			m.err = nil
			m.screen = screenConfirm
		}
	}
	return m, nil
}

func (m model) onConfirmKey(key string) (tea.Model, tea.Cmd) {
	if len(m.rescues) == 0 {
		m.screen = screenRescues
		return m, nil
	}
	selected := m.rescues[m.choice]

	switch key {
	case "esc", "n", "left", "h", "backspace":
		m.err = nil
		m.screen = screenRescues
	case "y":
		if m.discardsUncommitted() {
			m.err = errDirtyTree
			return m, nil
		}
		m.loading = true
		return m, applyCmd(m.session, *selected.plan)
	case "f":
		if !m.discardsUncommitted() {
			return m, nil
		}
		m.err = nil
		m.loading = true
		return m, applyCmd(m.session, *selected.plan)
	}
	return m, nil
}

func loadMoreCmd(s Session, limit int) tea.Cmd {
	return func() tea.Msg {
		events, err := timeline.Load(context.Background(), s.Git, limit)
		return eventsMsg{events: events, limit: limit, err: err}
	}
}

func detectCmd(s Session) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := &recipes.Repo{Git: s.Git, Events: s.Events, Printer: s.Printer}

		var found []rescue
		for _, r := range recipes.All() {
			plan, err := r.Detect(ctx, repo)
			if err != nil {
				return rescuesMsg{err: err}
			}
			if plan != nil {
				found = append(found, rescue{recipe: r, plan: plan})
			}
		}

		status, err := safety.WorkingTreeStatus(ctx, s.Git)
		if err != nil {
			return rescuesMsg{err: err}
		}
		return rescuesMsg{rescues: found, dirty: !status.Clean}
	}
}

func applyCmd(s Session, plan safety.Plan) tea.Cmd {
	return func() tea.Msg {
		res, err := safety.Apply(context.Background(), s.Git, plan, safety.Options{Execute: true, Now: time.Now()})
		return appliedMsg{result: res, err: err}
	}
}

func (m model) View() string {
	if m.help {
		return m.helpView()
	}
	switch m.screen {
	case screenDetail:
		return m.detailView()
	case screenRescues:
		return m.rescuesView()
	case screenConfirm:
		return m.confirmView()
	case screenResult:
		return m.resultView()
	default:
		return m.timelineView()
	}
}

func (m model) footer() string {
	var b strings.Builder
	b.WriteString("\n")
	if m.loading {
		b.WriteString(m.say(i18n.TuiWorking))
	}
	if m.err != nil {
		b.WriteString(warnStyle.Render(m.say(i18n.TuiError, m.errorText())) + "\n")
	}
	b.WriteString(helpStyle.Render(m.footerHint()))
	return b.String()
}

func (m model) errorText() string {
	if errors.Is(m.err, errDirtyTree) {
		return m.say(i18n.TuiErrDirtyTree)
	}
	var applyErr *safety.ApplyError
	if errors.As(m.err, &applyErr) {
		return m.say(i18n.TuiErrApplyFailed, applyErr.Command, applyErr.Err, applyErr.BackupBranch)
	}
	return m.err.Error()
}
