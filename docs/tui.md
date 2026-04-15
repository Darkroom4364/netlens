# TUI Reference

## Launching

```bash
netlens tui -t <topology.graphml> [-m <solver>]
```

`-t` (required) path to a Topology Zoo GraphML file. `-m` selects the solver
method (default `tikhonov`; options: `tsvd`, `tikhonov`, `nnls`, `admm`, `irl1`,
`vardi`, `tomogravity`, `laplacian`). The command simulates measurements, solves, and opens an interactive
full-screen terminal UI.

## Layout

The screen is split into three regions:

| Region | Panel | Content |
|--------|-------|---------|
| Main | **Tree / Heatmap** | Tree view: links grouped by source node with expand/collapse, bar charts, and color-coded delays. Heatmap view: matrix of per-link delays between node pairs. Toggle with `h`/`t`. |
| Bottom | **Detail Bar** | Detail for the currently selected link: delay, confidence interval, σ deviation, path coverage, identifiability status. |
| Footer | **Status Bar** | Keybinding hints, current sort/filter mode, solver name, routing-matrix rank, identifiable fraction. |

## Keybindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Navigate down |
| `k` / `↑` | Navigate up |
| `Enter` | Expand/collapse node |
| `h` | Heatmap view |
| `t` | Tree view |
| `/` | Filter by node name |
| `s` | Cycle sort mode |
| `m` | Cycle solver and re-solve |
| `?` | Help overlay |
| `q` | Quit |

## Color Coding

Link health in the topology list and heatmap bars uses three thresholds:

### Tree View

| Color | Condition |
|-------|-----------|
| Green | delay < 5 ms |
| Yellow | 5 ms ≤ delay ≤ 20 ms |
| Red | delay > 20 ms |

### Heatmap View

| Color | Condition |
|-------|-----------|
| Green | delay < 2 ms |
| Yellow | 2 ms ≤ delay ≤ 10 ms |
| Red | delay > 10 ms |

The selected link row is highlighted in the tree view.

## Auto-Refresh

When constructed with `NewWithRefresh`, the TUI re-solves the problem on a
timer. Each tick calls `solver.Solve(problem)` and replaces the current
solution, so the display updates live as underlying measurements change.

```go
model := tui.NewWithRefresh(problem, initialSolution, solvers, solverIdx, 5*time.Second)
p := tea.NewProgram(model, tea.WithAltScreen())
p.Run()
```

If `solvers` is nil, `NewWithRefresh` behaves identically to `New` (no
auto-refresh). The refresh rate is the `time.Duration` passed as the last
argument.

## Programmatic Use

Both constructors return a `tui.Model` that satisfies `bubbletea.Model`:

```go
// One-shot display of a pre-computed solution.
tui.New(problem, solution, solvers, solverIdx)

// Live-updating display that re-solves every interval.
tui.NewWithRefresh(problem, solution, solvers, solverIdx, rate)
```

Pass the model to `tea.NewProgram` with `tea.WithAltScreen()` for full-screen
mode.
