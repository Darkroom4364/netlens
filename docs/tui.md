# TUI Reference

## Launching

```bash
netlens tui -t <topology.graphml> [-m <solver>]
```

`-t` (required) path to a Topology Zoo GraphML file. `-m` selects the solver
method (default `tikhonov`; options: `tsvd`, `tikhonov`, `nnls`, `admm`,
`vardi`). The command simulates measurements, solves, and opens an interactive
full-screen terminal UI.

## Layout

The screen is split into three regions:

| Region | Panel | Content |
|--------|-------|---------|
| Left half | **Topology -- Links** | Scrollable list of all links with index, src->dst, and inferred delay (ms). Each row is color-coded by health. |
| Right half | **Link Detail** | Detail for the currently selected link: delay, confidence interval, identifiability status, path coverage count, and the list of measurement paths traversing the link. |
| Bottom | **Status Bar** | Solver method, solve duration, routing-matrix rank vs. link count, identifiable fraction, and a keybinding reminder. |

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

| Color | Condition |
|-------|-----------|
| Green | delay < 5 ms |
| Yellow | 5 ms <= delay < 20 ms |
| Red | delay >= 20 ms |
| Dim (faint) | Link is not identifiable |

The selected link row is rendered bold + reverse video.

## Heatmap Panel

`RenderHeatmap` produces a bar-chart view of all links sorted worst-first.
Each row shows `src->dst`, a filled block bar scaled to the maximum
identifiable delay, and the numeric delay. Bars use the same green/yellow/red
thresholds; unidentifiable links are dimmed. The heatmap is available for
programmatic embedding but is not wired into the default two-panel TUI layout.

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
