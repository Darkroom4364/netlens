//go:build tui

package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Darkroom4364/netlens/tomo"
)

type viewMode int

const (
	viewTree viewMode = iota
	viewHeatmap
)

// tickMsg is sent on each refresh interval to trigger a re-solve.
type tickMsg time.Time

// solveResultMsg carries the result of an async solve.
type solveResultMsg struct{ sol *tomo.Solution }

// Model is the top-level Bubble Tea model for the netlens TUI.
type Model struct {
	problem     *tomo.Problem
	solution    *tomo.Solution
	width       int
	height      int
	mode        viewMode
	cursor      int // flat row index in tree view
	expanded    map[int]bool
	solvers     []tomo.Solver
	solverIdx   int
	refreshRate time.Duration
	showHelp    bool
	filtering   bool
	filterText  string
	sortMode    int
}

// New creates a new TUI model with a list of available solvers.
func New(p *tomo.Problem, s *tomo.Solution, solvers []tomo.Solver, solverIdx int) Model {
	return Model{
		problem:   p,
		solution:  s,
		solvers:   solvers,
		solverIdx: solverIdx,
		expanded:  make(map[int]bool),
	}
}

// NewWithRefresh creates a TUI model that re-solves on a timer.
func NewWithRefresh(p *tomo.Problem, s *tomo.Solution, solvers []tomo.Solver, solverIdx int, rate time.Duration) Model {
	return Model{
		problem:     p,
		solution:    s,
		solvers:     solvers,
		solverIdx:   solverIdx,
		refreshRate: rate,
		expanded:    make(map[int]bool),
	}
}

func (m Model) Init() tea.Cmd {
	if m.refreshRate > 0 && len(m.solvers) > 0 {
		return tea.Tick(m.refreshRate, func(t time.Time) tea.Msg { return tickMsg(t) })
	}
	return nil
}

func (m Model) currentSolver() tomo.Solver {
	if len(m.solvers) == 0 {
		return nil
	}
	return m.solvers[m.solverIdx%len(m.solvers)]
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// In filter mode, capture text input.
		if m.filtering {
			switch msg.String() {
			case "enter":
				m.filtering = false
				if max := TreeRowCount(m.problem, m.solution, m.expanded, m.filterText, m.sortMode) - 1; m.cursor > max {
					m.cursor = max
				}
			case "esc":
				m.filtering = false
				m.filterText = ""
				if max := TreeRowCount(m.problem, m.solution, m.expanded, "", m.sortMode) - 1; m.cursor > max {
					m.cursor = max
				}
			case "backspace":
				if len(m.filterText) > 0 {
					m.filterText = m.filterText[:len(m.filterText)-1]
				}
			default:
				if len(msg.String()) == 1 {
					m.filterText += msg.String()
				}
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < TreeRowCount(m.problem, m.solution, m.expanded, m.filterText, m.sortMode)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if nid := CursorToNodeID(m.problem, m.solution, m.cursor, m.expanded, m.filterText, m.sortMode); nid >= 0 {
				m.expanded[nid] = !m.expanded[nid]
			}
		case "h":
			m.mode = viewHeatmap
		case "t":
			m.mode = viewTree
		case "/":
			m.filtering = true
		case "s":
			m.sortMode = (m.sortMode + 1) % 5
			if max := TreeRowCount(m.problem, m.solution, m.expanded, m.filterText, m.sortMode) - 1; m.cursor > max {
				m.cursor = max
			}
		case "m":
			if len(m.solvers) > 0 {
				m.solverIdx = (m.solverIdx + 1) % len(m.solvers)
				solver := m.currentSolver()
				p := m.problem
				return m, func() tea.Msg {
					sol, err := solver.Solve(p)
					if err != nil {
						return nil
					}
					return solveResultMsg{sol}
				}
			}
		case "?":
			m.showHelp = !m.showHelp
		case "esc":
			m.showHelp = false
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case solveResultMsg:
		if msg.sol != nil {
			m.solution = msg.sol
		}
	case tickMsg:
		if solver := m.currentSolver(); solver != nil {
			p := m.problem
			return m, tea.Batch(
				func() tea.Msg {
					sol, err := solver.Solve(p)
					if err != nil {
						return nil
					}
					return solveResultMsg{sol}
				},
				tea.Tick(m.refreshRate, func(t time.Time) tea.Msg { return tickMsg(t) }),
			)
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	if m.showHelp {
		return RenderHelpOverlay(m.width, m.height)
	}

	// Reserve space: 1 for status bar, 3 for detail bar, 1 padding.
	statusH := 1
	detailH := 3
	mainH := m.height - statusH - detailH - 1
	if mainH < 1 {
		mainH = 1
	}

	// Map cursor to link index for detail bar and heatmap.
	linkIdx := CursorToLinkIdx(m.problem, m.solution, m.cursor, m.expanded, m.filterText, m.sortMode)

	// Main panel.
	var main string
	switch m.mode {
	case viewHeatmap:
		main = RenderHeatmapView(m.problem, m.solution, linkIdx, m.filterText, m.width, mainH)
	default:
		main = RenderTreeView(m.problem, m.solution, m.cursor, m.expanded, m.filterText, m.sortMode, m.width, mainH)
	}
	main = lipgloss.NewStyle().Width(m.width).Height(mainH).Render(main)

	// Detail bar.
	detail := RenderDetailBar(m.problem, m.solution, linkIdx, m.width)
	detail = lipgloss.NewStyle().Width(m.width).Render(detail)

	// Status bar.
	solverName := ""
	if solver := m.currentSolver(); solver != nil {
		solverName = solver.Name()
	}
	status := RenderStatusBar(m.problem, m.solution, m.mode, solverName, m.filtering, m.filterText, m.sortMode, m.width)
	status = lipgloss.NewStyle().Width(m.width).Render(status)

	return lipgloss.JoinVertical(lipgloss.Left, main, detail, status)
}


