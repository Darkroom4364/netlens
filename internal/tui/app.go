//go:build tui

package tui

import (
	"fmt"
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

// Model is the top-level Bubble Tea model for the netlens TUI.
type Model struct {
	problem      *tomo.Problem
	solution     *tomo.Solution
	width        int
	height       int
	mode         viewMode
	selectedNode int
	selectedLink int
	expanded     map[int]bool
	solver       tomo.Solver
	refreshRate  time.Duration
}

// New creates a new TUI model.
func New(p *tomo.Problem, s *tomo.Solution) Model {
	return Model{
		problem:  p,
		solution: s,
		expanded: make(map[int]bool),
	}
}

// NewWithRefresh creates a TUI model that re-solves on a timer.
func NewWithRefresh(p *tomo.Problem, s *tomo.Solution, solver tomo.Solver, rate time.Duration) Model {
	return Model{
		problem:     p,
		solution:    s,
		solver:      solver,
		refreshRate: rate,
		expanded:    make(map[int]bool),
	}
}

func (m Model) Init() tea.Cmd {
	if m.solver != nil {
		return tea.Tick(m.refreshRate, func(t time.Time) tea.Msg { return tickMsg(t) })
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.selectedLink < m.problem.NumLinks()-1 {
				m.selectedLink++
			}
		case "k", "up":
			if m.selectedLink > 0 {
				m.selectedLink--
			}
		case "enter":
			m.expanded[m.selectedLink] = !m.expanded[m.selectedLink]
		case "h":
			m.mode = viewHeatmap
		case "t":
			m.mode = viewTree
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		if m.solver != nil {
			if sol, err := m.solver.Solve(m.problem); err == nil {
				m.solution = sol
			}
			return m, tea.Tick(m.refreshRate, func(t time.Time) tea.Msg { return tickMsg(t) })
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	// Reserve space: 1 for status bar, 3 for detail bar, 1 padding.
	statusH := 1
	detailH := 3
	mainH := m.height - statusH - detailH - 1
	if mainH < 1 {
		mainH = 1
	}

	// Main panel.
	var main string
	switch m.mode {
	case viewHeatmap:
		main = renderHeatmap(m.problem, m.solution, m.selectedLink, m.width, mainH)
	default:
		main = renderTreeView(m.problem, m.solution, m.selectedLink, m.expanded, m.width, mainH)
	}
	main = lipgloss.NewStyle().Width(m.width).Height(mainH).Render(main)

	// Detail bar.
	detail := RenderDetailBar(m.problem, m.solution, m.selectedLink, m.width)
	detail = lipgloss.NewStyle().Width(m.width).Render(detail)

	// Status bar.
	solverName := ""
	if m.solver != nil {
		solverName = m.solver.Name()
	}
	status := RenderStatusBar(m.problem, m.solution, m.mode, solverName, m.width)
	status = lipgloss.NewStyle().Width(m.width).Render(status)

	return lipgloss.JoinVertical(lipgloss.Left, main, detail, status)
}

// ---------------------------------------------------------------------------
// Placeholder render functions — Wave 2 will replace these with real panels.
// ---------------------------------------------------------------------------

func renderTreeView(_ *tomo.Problem, _ *tomo.Solution, _ int, _ map[int]bool, w, h int) string {
	return fmt.Sprintf("Tree View (%dx%d)", w, h)
}

func renderHeatmap(_ *tomo.Problem, _ *tomo.Solution, _ int, w, h int) string {
	return fmt.Sprintf("Heatmap (%dx%d)", w, h)
}

