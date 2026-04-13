//go:build tui

package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/internal/tui/panels"
	"github.com/Darkroom4364/netlens/internal/tui/styles"
)

// tickMsg is sent on each refresh interval to trigger a re-solve.
type tickMsg time.Time

// Model is the top-level Bubble Tea model for the netlens TUI.
type Model struct {
	problem      *tomo.Problem
	solution     *tomo.Solution
	selectedLink int
	width        int
	height       int
	solver       tomo.Solver
	refreshRate  time.Duration
}

// New creates a new TUI model.
func New(p *tomo.Problem, s *tomo.Solution) Model {
	return Model{problem: p, solution: s}
}

// NewWithRefresh creates a TUI model that re-solves on a timer.
// If solver is nil, behaves identically to New.
func NewWithRefresh(p *tomo.Problem, s *tomo.Solution, solver tomo.Solver, rate time.Duration) Model {
	return Model{problem: p, solution: s, solver: solver, refreshRate: rate}
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
		case "up", "k":
			if m.selectedLink > 0 {
				m.selectedLink--
			}
		case "down", "j":
			if m.selectedLink < m.problem.NumLinks()-1 {
				m.selectedLink++
			}
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

	leftW := m.width/2 - 2
	rightW := m.width - leftW - 4
	bodyH := m.height - 2

	left := styles.Panel.Width(leftW).Height(bodyH).Render(
		panels.RenderTopology(m.problem, m.solution, m.selectedLink, leftW, bodyH),
	)
	right := styles.Panel.Width(rightW).Height(bodyH).Render(
		panels.RenderResults(m.problem, m.solution, m.selectedLink, rightW, bodyH),
	)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	status := panels.RenderStatusbar(m.problem, m.solution, m.width)

	return lipgloss.JoinVertical(lipgloss.Left, body, status)
}
