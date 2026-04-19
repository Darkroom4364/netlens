//go:build tui

package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Darkroom4364/netlens/internal/measure"
	"github.com/Darkroom4364/netlens/internal/tui/styles"
	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/topology"
)

// appPhase tracks which screen of the TUI is active.
type appPhase int

const (
	phaseSourceSelect appPhase = iota
	phaseSourceConfig
	phaseSolverSelect
	phaseLoading
	phaseDashboard
)

// dataSource identifies which data source the user selected.
type dataSource int

const (
	sourceSimulate dataSource = iota
	sourceRIPEAtlas
	sourceTraceroute
)

var sourceLabels = []string{
	"Simulate (bundled topology)",
	"RIPE Atlas (measurement ID)",
	"Traceroute file (JSON)",
}

var bundledTopologies = []string{
	"abilene", "attmpls", "btnorthamerica", "cesnet200706",
	"chinanet", "claranet", "dfn", "geant2012", "karen", "sprint",
}

type solverInfo struct {
	name string
	desc string
}

var solverList = []solverInfo{
	{"tikhonov", "general-purpose, good default"},
	{"tsvd", "fast, best for clean data"},
	{"nnls", "enforces non-negative delays"},
	{"admm", "sparse networks, few congested links"},
	{"irl1", "precise sparsity recovery, slow"},
	{"vardi", "underdetermined systems, many paths"},
	{"tomogravity", "gravity prior + correction"},
	{"laplacian", "topology-aware, smooth solutions"},
}

type knownMSM struct {
	id   int
	desc string
}

var knownMeasurements = []knownMSM{
	{5001, "k.root (RIPE NCC)"},
	{5004, "b.root (USC-ISI)"},
	{5005, "c.root (Cogent)"},
	{5006, "d.root (Univ. of Maryland)"},
	{5008, "e.root (NASA Ames)"},
	{5010, "g.root (US DoD NIC)"},
	{5013, "j.root (Verisign)"},
	{5015, "l.root (ICANN)"},
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// --- Messages ---

type loadResultMsg struct {
	problem   *tomo.Problem
	solution  *tomo.Solution
	solvers   []tomo.Solver
	solverIdx int
	err       error
}

type spinnerTickMsg struct{}
type logLineMsg string

func spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// listenForProgress reads one line from the progress channel and sends it
// as a logLineMsg. Returns nil when the channel is closed.
func listenForProgress(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return nil
		}
		return logLineMsg(line)
	}
}

// --- Async load commands ---

func allSolvers() []tomo.Solver {
	return []tomo.Solver{
		&tomo.TikhonovSolver{},
		&tomo.TSVDSolver{},
		&tomo.NNLSSolver{},
		&tomo.ADMMSolver{},
		&tomo.IRL1Solver{},
		&tomo.VardiEMSolver{},
		&tomo.TomogravitySolver{},
		&tomo.LaplacianSolver{},
	}
}

func loadSimulateCmd(ctx context.Context, progCh chan<- string, topoPath string, solverIdx int) tea.Cmd {
	return func() tea.Msg {
		defer close(progCh)

		progCh <- "Loading topology..."
		g, err := topology.LoadGraphML(topoPath)
		if err != nil {
			return loadResultMsg{err: fmt.Errorf("load topology: %w", err)}
		}
		if ctx.Err() != nil {
			return loadResultMsg{err: ctx.Err()}
		}

		progCh <- "Simulating measurements..."
		sim, err := measure.Simulate(g, measure.DefaultSimConfig())
		if err != nil {
			return loadResultMsg{err: fmt.Errorf("simulate: %w", err)}
		}
		if ctx.Err() != nil {
			return loadResultMsg{err: ctx.Err()}
		}

		solvers := allSolvers()
		progCh <- fmt.Sprintf("Solving with %s...", solvers[solverIdx].Name())
		sol, err := solvers[solverIdx].Solve(sim.Problem)
		if err != nil {
			return loadResultMsg{err: fmt.Errorf("solve: %w", err)}
		}
		progCh <- "Done"
		return loadResultMsg{
			problem:   sim.Problem,
			solution:  sol,
			solvers:   solvers,
			solverIdx: solverIdx,
		}
	}
}

func loadRIPEAtlasCmd(ctx context.Context, progCh chan<- string, msmID int, solverIdx int) tea.Cmd {
	return func() tea.Msg {
		defer close(progCh)

		progCh <- "Connecting to RIPE Atlas API..."
		apiKey := os.Getenv("RIPE_ATLAS_API_KEY")
		src := measure.NewRIPEAtlasSource(apiKey, "", nil)
		now := time.Now().Unix()

		progCh <- fmt.Sprintf("Fetching results for MSM %d...", msmID)
		measurements, err := src.FetchResults(ctx, msmID, now-3600, now)
		if err != nil {
			return loadResultMsg{err: fmt.Errorf("fetch RIPE Atlas: %w", err)}
		}
		if len(measurements) == 0 {
			return loadResultMsg{err: fmt.Errorf("no measurements returned for MSM %d", msmID)}
		}
		if ctx.Err() != nil {
			return loadResultMsg{err: ctx.Err()}
		}

		progCh <- fmt.Sprintf("Loaded %d measurements", len(measurements))
		return buildFromMeasurements(ctx, progCh, measurements, solverIdx)
	}
}

func loadTracerouteCmd(ctx context.Context, progCh chan<- string, filePath string, solverIdx int) tea.Cmd {
	return func() tea.Msg {
		defer close(progCh)

		progCh <- "Reading traceroute file..."
		data, err := os.ReadFile(filePath)
		if err != nil {
			return loadResultMsg{err: fmt.Errorf("read file: %w", err)}
		}
		if ctx.Err() != nil {
			return loadResultMsg{err: ctx.Err()}
		}

		progCh <- "Parsing measurements..."
		measurements, err := measure.ParseRIPEAtlasTraceroute(data)
		if err != nil || len(measurements) == 0 {
			measurements, err = measure.ParseScamperJSON(data)
		}
		if err != nil {
			return loadResultMsg{err: fmt.Errorf("parse traceroute: %w", err)}
		}
		if len(measurements) == 0 {
			return loadResultMsg{err: fmt.Errorf("no measurements in file")}
		}

		progCh <- fmt.Sprintf("Loaded %d measurements", len(measurements))
		return buildFromMeasurements(ctx, progCh, measurements, solverIdx)
	}
}

func buildFromMeasurements(ctx context.Context, progCh chan<- string, measurements []tomo.PathMeasurement, solverIdx int) loadResultMsg {
	progCh <- "Inferring topology..."
	graph, pathSpecs, acceptedIdx, err := topology.InferFromMeasurements(measurements, topology.InferOpts{
		MaxAnonymousFrac: 0.3,
	})
	if err != nil {
		return loadResultMsg{err: fmt.Errorf("infer topology: %w", err)}
	}
	if ctx.Err() != nil {
		return loadResultMsg{err: ctx.Err()}
	}

	accepted := make([]tomo.PathMeasurement, len(acceptedIdx))
	for i, idx := range acceptedIdx {
		accepted[i] = measurements[idx]
	}

	progCh <- "Building routing matrix..."
	problem, err := tomo.BuildProblemFromMeasurements(graph, accepted, pathSpecs)
	if err != nil {
		return loadResultMsg{err: fmt.Errorf("build problem: %w", err)}
	}
	if ctx.Err() != nil {
		return loadResultMsg{err: ctx.Err()}
	}

	solvers := allSolvers()
	progCh <- fmt.Sprintf("Solving with %s...", solvers[solverIdx].Name())
	sol, err := solvers[solverIdx].Solve(problem)
	if err != nil {
		return loadResultMsg{err: fmt.Errorf("solve: %w", err)}
	}
	progCh <- "Done"
	return loadResultMsg{
		problem:   problem,
		solution:  sol,
		solvers:   solvers,
		solverIdx: solverIdx,
	}
}

// --- Update helpers ---

func (m Model) updateSourceSelect(msg tea.Msg) (Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.wizCursor < len(sourceLabels)-1 {
			m.wizCursor++
		}
	case "k", "up":
		if m.wizCursor > 0 {
			m.wizCursor--
		}
	case "enter":
		m.source = dataSource(m.wizCursor)
		m.wizCursor = 0
		m.phase = phaseSourceConfig
		// For RIPE Atlas and traceroute, activate text input immediately.
		if m.source == sourceRIPEAtlas || m.source == sourceTraceroute {
			m.inputActive = true
		}
	}
	return m, nil
}

func (m Model) updateSourceConfig(msg tea.Msg) (Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	// Text input mode for RIPE Atlas MSM ID or traceroute file path.
	if m.inputActive {
		switch km.String() {
		case "enter":
			if m.source == sourceRIPEAtlas {
				if _, err := strconv.Atoi(m.msmIDInput); err != nil || m.msmIDInput == "" {
					m.loadErr = "Measurement ID must be a number"
					return m, nil
				}
			} else if m.source == sourceTraceroute && m.filePathInput == "" {
				m.loadErr = "File path is required"
				return m, nil
			}
			m.loadErr = ""
			m.inputActive = false
			m.wizCursor = 0
			m.phase = phaseSolverSelect
		case "esc":
			m.inputActive = false
			m.msmIDInput = ""
			m.filePathInput = ""
			m.loadErr = ""
			m.wizCursor = 0
			m.phase = phaseSourceSelect
		case "backspace":
			if m.source == sourceRIPEAtlas && len(m.msmIDInput) > 0 {
				m.msmIDInput = m.msmIDInput[:len(m.msmIDInput)-1]
			} else if m.source == sourceTraceroute && len(m.filePathInput) > 0 {
				m.filePathInput = m.filePathInput[:len(m.filePathInput)-1]
			}
			m.loadErr = ""
		default:
			ch := km.String()
			if len(ch) == 1 {
				if m.source == sourceRIPEAtlas {
					m.msmIDInput += ch
				} else {
					m.filePathInput += ch
				}
				m.loadErr = ""
			}
		}
		return m, nil
	}

	// List selection mode (Simulate topology picker).
	switch km.String() {
	case "esc":
		m.wizCursor = 0
		m.phase = phaseSourceSelect
	case "j", "down":
		if m.wizCursor < len(bundledTopologies) {
			m.wizCursor++
		}
	case "k", "up":
		if m.wizCursor > 0 {
			m.wizCursor--
		}
	case "enter":
		if m.wizCursor < len(bundledTopologies) {
			m.topoChoice = m.wizCursor
		} else {
			// "Custom path..." selected — switch to text input.
			m.inputActive = true
			m.source = sourceTraceroute // reuse traceroute text input
			return m, nil
		}
		m.wizCursor = 0
		m.phase = phaseSolverSelect
	}
	return m, nil
}

func (m Model) updateSolverSelect(msg tea.Msg) (Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "esc":
		m.wizCursor = 0
		m.phase = phaseSourceConfig
		if m.source == sourceRIPEAtlas || m.source == sourceTraceroute {
			m.inputActive = true
		}
	case "j", "down":
		if m.wizCursor < len(solverList)-1 {
			m.wizCursor++
		}
	case "k", "up":
		if m.wizCursor > 0 {
			m.wizCursor--
		}
	case "enter":
		m.solverIdx = m.wizCursor
		m.phase = phaseLoading
		m.spinnerFrame = 0
		m.loadErr = ""
		m.loadLogs = nil

		ctx, cancel := context.WithCancel(context.Background())
		m.cancelLoad = cancel
		progCh := make(chan string, 10)
		m.progCh = progCh

		var loadCmd tea.Cmd
		switch m.source {
		case sourceSimulate:
			topoPath := filepath.Join("testdata", "topologies", bundledTopologies[m.topoChoice]+".graphml")
			m.loadingMsg = fmt.Sprintf("Loading %s...", bundledTopologies[m.topoChoice])
			loadCmd = loadSimulateCmd(ctx, progCh, topoPath, m.solverIdx)
		case sourceRIPEAtlas:
			msmID, _ := strconv.Atoi(m.msmIDInput)
			m.loadingMsg = fmt.Sprintf("Fetching MSM %d...", msmID)
			loadCmd = loadRIPEAtlasCmd(ctx, progCh, msmID, m.solverIdx)
		case sourceTraceroute:
			m.loadingMsg = fmt.Sprintf("Loading %s...", filepath.Base(m.filePathInput))
			loadCmd = loadTracerouteCmd(ctx, progCh, m.filePathInput, m.solverIdx)
		}
		return m, tea.Batch(loadCmd, listenForProgress(progCh), spinnerTick())
	}
	return m, nil
}

func (m Model) updateLoading(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadResultMsg:
		if msg.err != nil {
			// Don't show "context canceled" as an error — it's an abort.
			if ctx_err := context.Canceled; msg.err == ctx_err || msg.err.Error() == ctx_err.Error() {
				return m, nil
			}
			m.loadErr = msg.err.Error()
			return m, nil
		}
		m.problem = msg.problem
		m.solution = msg.solution
		m.solvers = msg.solvers
		m.solverIdx = msg.solverIdx
		m.expanded = make(map[int]bool)
		m.phase = phaseDashboard
		return m, nil
	case logLineMsg:
		m.loadLogs = append(m.loadLogs, string(msg))
		return m, listenForProgress(m.progCh)
	case spinnerTickMsg:
		if m.loadErr != "" {
			return m, nil // stop spinner on error
		}
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		return m, spinnerTick()
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.loadErr != "" {
				// On error screen, go back to solver select.
				m.loadErr = ""
				m.loadLogs = nil
				m.wizCursor = 0
				m.phase = phaseSolverSelect
			} else if m.cancelLoad != nil {
				// Abort in-progress load.
				m.cancelLoad()
				m.loadErr = ""
				m.loadLogs = nil
				m.wizCursor = 0
				m.phase = phaseSolverSelect
			}
		case "q", "ctrl+c":
			if m.cancelLoad != nil {
				m.cancelLoad()
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

// --- Render helpers ---

func renderSourceSelect(m Model) string {
	var b strings.Builder
	b.WriteString(styles.WizardTitle.Render("netlens — Network Tomography"))
	b.WriteString("\n\n")
	b.WriteString("  Select data source:\n\n")

	for i, label := range sourceLabels {
		cursor := "  "
		if i == m.wizCursor {
			cursor = styles.WizardSelected.Render("▸ ")
			label = styles.WizardSelected.Render(label)
		}
		b.WriteString(fmt.Sprintf("  %s%s\n", cursor, label))
	}

	b.WriteString("\n  ")
	b.WriteString(styles.Dim.Render("↑/↓ navigate  Enter select  q quit"))

	box := styles.Panel.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func renderSourceConfig(m Model) string {
	var b strings.Builder

	switch m.source {
	case sourceSimulate:
		b.WriteString(styles.WizardTitle.Render("Select topology"))
		b.WriteString("\n\n")
		for i, name := range bundledTopologies {
			cursor := "  "
			label := name
			if i == m.wizCursor {
				cursor = styles.WizardSelected.Render("▸ ")
				label = styles.WizardSelected.Render(name)
			}
			b.WriteString(fmt.Sprintf("  %s%s\n", cursor, label))
		}
		// Custom path option.
		cursor := "  "
		label := "Custom path..."
		if m.wizCursor == len(bundledTopologies) {
			cursor = styles.WizardSelected.Render("▸ ")
			label = styles.WizardSelected.Render("Custom path...")
		}
		b.WriteString(fmt.Sprintf("  %s%s\n", cursor, label))

	case sourceRIPEAtlas:
		b.WriteString(styles.WizardTitle.Render("RIPE Atlas Configuration"))
		b.WriteString("\n\n")
		if os.Getenv("RIPE_ATLAS_API_KEY") != "" {
			b.WriteString("  API Key: " + styles.Green.Render("set (from env)") + "\n")
		} else {
			b.WriteString("  API Key: " + styles.Yellow.Render("not set") + "\n")
			b.WriteString("  " + styles.Dim.Render("Set RIPE_ATLAS_API_KEY env var for private measurements") + "\n")
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Measurement ID: %s█\n", m.msmIDInput))
		b.WriteString("\n")
		b.WriteString("  " + styles.Dim.Render("Well-known traceroute measurements:") + "\n")
		for _, msm := range knownMeasurements {
			b.WriteString(fmt.Sprintf("    %s%-6d%s %s\n",
				styles.Dim.Render(""),
				msm.id,
				styles.Dim.Render(" —"),
				styles.Dim.Render(msm.desc)))
		}

	case sourceTraceroute:
		b.WriteString(styles.WizardTitle.Render("Traceroute File"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  File path: %s█\n", m.filePathInput))
	}

	if m.loadErr != "" {
		b.WriteString("\n  " + styles.WizardError.Render(m.loadErr))
	}

	b.WriteString("\n\n  ")
	b.WriteString(styles.Dim.Render("Enter confirm  Esc back"))

	box := styles.Panel.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func renderSolverSelect(m Model) string {
	var b strings.Builder
	b.WriteString(styles.WizardTitle.Render("Select solver"))
	b.WriteString("\n\n")

	for i, s := range solverList {
		cursor := "  "
		label := fmt.Sprintf("%-14s %s", s.name, styles.Dim.Render(s.desc))
		if i == m.wizCursor {
			cursor = styles.WizardSelected.Render("▸ ")
			label = styles.WizardSelected.Render(fmt.Sprintf("%-14s", s.name)) + " " + styles.Dim.Render(s.desc)
		}
		b.WriteString(fmt.Sprintf("  %s%s\n", cursor, label))
	}

	b.WriteString("\n  ")
	b.WriteString(styles.Dim.Render("↑/↓ navigate  Enter select  Esc back"))

	box := styles.Panel.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func renderLoading(m Model) string {
	var b strings.Builder

	if m.loadErr != "" {
		b.WriteString(styles.WizardError.Render("  ✗ " + m.loadErr))
		b.WriteString("\n")
		if len(m.loadLogs) > 0 {
			b.WriteString("\n")
			for _, line := range m.loadLogs {
				b.WriteString("  " + styles.Dim.Render(line) + "\n")
			}
		}
		b.WriteString("\n  ")
		b.WriteString(styles.Dim.Render("Esc back  q quit"))
	} else {
		frame := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
		b.WriteString(styles.WizardTitle.Render(fmt.Sprintf("  %s %s", frame, m.loadingMsg)))
		if len(m.loadLogs) > 0 {
			b.WriteString("\n")
			for _, line := range m.loadLogs {
				b.WriteString("\n  " + styles.Dim.Render(line))
			}
		}
		b.WriteString("\n\n  ")
		b.WriteString(styles.Dim.Render("Esc abort"))
	}

	box := styles.Panel.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
