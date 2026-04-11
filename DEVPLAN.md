# netlens — Development Plan

## Project Summary

**netlens** is a modern, general-purpose network tomography toolkit written in Go. It infers internal network state (per-link latency, loss, congestion) from edge measurements alone — without access to internal routers. It is the first usable implementation of network tomography since a dead R package from 2012.

**One binary. Feed it traceroutes. See the invisible.**

## Goals

1. **Portfolio piece** — demonstrates applied math (linear algebra, inverse problems), systems programming (Go), and networking depth. Targets master's applications and infrastructure engineering roles.
2. **First modern implementation** — no Python package on PyPI, no Go/Rust tool, dead R package. Fill a real gap.
3. **Publishable** — benchmark framework comparing classical methods on real topologies, plus a measurement design contribution, could yield a workshop paper (IMC, PAM, CoNEXT).
4. **Usable** — not a Jupyter notebook. A real CLI + TUI tool that works with real data sources.

## Non-Goals

- Not a business / not optimizing for revenue
- Not competing with Datadog/ThousandEyes/Kentik (they use active probing, not tomography)
- Not SCION-specific (killed in adversarial testing — SCION is too small and already has direct per-hop measurement)

---

## Architecture

### Layered Design

```
┌───────────────────────────────────────────────────────┐
│  Frontends                                            │
│  ├── CLI     (netlens scan/simulate/benchmark/plan)   │
│  ├── TUI     (real-time topology + heatmap)           │
│  └── Library (import as Go package)                   │
├───────────────────────────────────────────────────────┤
│  Analysis layer                                       │
│  ├── Identifiability analysis (rank, null space)      │
│  ├── Matrix quality scoring (condition number, cov.)  │
│  ├── Measurement design (optimal probe selection)     │
│  ├── Anomaly detection                                │
│  └── Simulation / benchmarking                        │
├───────────────────────────────────────────────────────┤
│  Inference engine (core)                              │
│  │                                                    │
│  │  Link delay / loss estimation (Ax = b):            │
│  │  ├── Tikhonov regularized least squares            │
│  │  ├── NNLS (non-negative least squares)             │
│  │  ├── Truncated SVD (TSVD)                          │
│  │  ├── ADMM (L1 / compressed sensing)                │
│  │  └── [Phase 3] ML via ONNX                         │
│  │                                                    │
│  │  Traffic matrix estimation (Aᵀx = b):              │
│  │  ├── Vardi EM                                      │
│  │  └── Tomogravity                                   │
│  │                                                    │
├───────────────────────────────────────────────────────┤
│  Data adapters (MeasurementSource interface)           │
│  ├── RIPE Atlas API (streaming, rate-limited)         │
│  ├── Traceroute parsers (scamper, mtr, paris)         │
│  ├── PerfSONAR                                        │
│  ├── ICMP ping (pro-bing)                             │
│  ├── Simulated (Topology Zoo + synthetic noise)       │
│  └── [Phase 3] Cloud APIs (AWS/GCP latency)           │
└───────────────────────────────────────────────────────┘
```

### Project Structure

```
netlens/
├── cmd/netlens/main.go                  # Cobra entrypoint
├── internal/
│   ├── tomo/                            # Core inference engine
│   │   ├── types.go                     # Link, Path, Measurement, Topology interface
│   │   ├── problem.go                   # Problem, Solution, Solver interface
│   │   ├── routing.go                   # Build routing matrix A from topology + paths
│   │   ├── identity.go                  # Identifiability analysis (rank, null space, cond. number)
│   │   ├── quality.go                   # Matrix quality score (coverage, identifiable fraction)
│   │   ├── tikhonov.go                  # Tikhonov regularized least squares (L-curve / GCV)
│   │   ├── nnls.go                      # Lawson-Hanson NNLS (active-set, QR-based inner solve)
│   │   ├── tsvd.go                      # Truncated SVD with discrepancy principle
│   │   ├── admm.go                      # ADMM for L1-minimized compressed sensing
│   │   ├── vardi.go                     # Vardi EM for traffic matrix estimation
│   │   ├── tomogravity.go              # Tomogravity (gravity prior + regularized LS)
│   │   └── solver_test.go              # Validate all solvers against known solutions
│   ├── topology/
│   │   ├── graph.go                     # Network graph (gonum/graph), implements tomo.Topology
│   │   ├── infer.go                     # Topology inference from traceroutes
│   │   ├── zoo.go                       # Topology Zoo GraphML loader
│   │   ├── synthetic.go                # Barabási-Albert, Waxman random graph generators
│   │   ├── alias.go                     # IP alias resolution (analytical Kapar)
│   │   └── topology_test.go
│   ├── measure/
│   │   ├── source.go                    # MeasurementSource interface (streaming/iterator)
│   │   ├── traceroute.go               # Parse scamper JSON, mtr JSON, paris-traceroute
│   │   ├── ripe.go                      # RIPE Atlas REST API client (rate-limited, paginated)
│   │   ├── perfsonar.go                # PerfSONAR esmond API adapter
│   │   ├── ping.go                      # ICMP probing via pro-bing
│   │   ├── simulate.go                 # Synthetic measurement generator
│   │   ├── cache.go                     # Local cache for fetched measurements (SQLite/bbolt)
│   │   └── measure_test.go
│   ├── plan/
│   │   ├── design.go                    # Measurement design: recommend optimal probe pairs
│   │   └── greedy.go                   # Greedy rank-maximizing probe selection
│   ├── bench/
│   │   ├── bench.go                     # Benchmark runner (all solvers × all topologies)
│   │   ├── metrics.go                   # RMSE, MAE, relative error, rank correlation, detection rate
│   │   └── report.go                   # Generate comparison tables + charts
│   ├── format/
│   │   ├── json.go                      # JSON output
│   │   ├── csv.go                       # CSV output
│   │   ├── dot.go                       # Pure Go DOT generation (no CGo)
│   │   └── format.go                   # Format dispatcher
│   ├── cli/
│   │   ├── root.go                      # Cobra root (no args → TUI)
│   │   ├── scan.go                      # netlens scan --source ripe --method tikhonov
│   │   ├── simulate.go                 # netlens simulate --topology abilene --noise 0.1
│   │   ├── benchmark.go                # netlens benchmark --topologies all --methods all
│   │   └── plan.go                      # netlens plan --source ripe --budget 50
│   └── tui/
│       ├── app.go                       # Bubble Tea root model
│       ├── keymap.go                    # Keybindings
│       ├── styles.go                    # Lipgloss styles
│       ├── commands.go                  # tea.Cmd producers (background solve, data fetch)
│       └── panels/
│           ├── topology.go             # Network graph view (ASCII box drawing)
│           ├── heatmap.go              # Per-link latency/loss heatmap
│           ├── timeseries.go           # Live measurement feed
│           ├── results.go              # Inference results + comparison table
│           └── statusbar.go            # Mode, hints, solver status, matrix quality
├── testdata/
│   ├── topologies/                     # Topology Zoo GraphML + synthetic configs
│   │   ├── abilene.graphml
│   │   ├── geant.graphml
│   │   └── ... (10-15 representative topologies)
│   └── measurements/                   # Sample real-world data
│       ├── ripe_atlas_sample.json
│       └── scamper_sample.json
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yaml
├── .github/workflows/ci.yml
├── LICENSE                              # Apache 2.0
└── README.md
```

### Core Interfaces

```go
// internal/tomo/types.go

// Topology is the interface for network graph access.
// Defined here to avoid circular imports with the topology package.
type Topology interface {
    NumNodes() int
    NumLinks() int
    Links() []Link
    PathLinks(src, dst int) ([]int, bool)  // link indices on shortest path
}

// Link represents a network link between two nodes.
type Link struct {
    ID     int
    Src    int
    Dst    int
    SrcIP  string  // optional: real IP if known
    DstIP  string
}

// PathMeasurement is a single end-to-end observation.
type PathMeasurement struct {
    Src       string
    Dst       string
    Hops      []Hop
    RTTs      []time.Duration   // multiple samples per path
    Loss      float64
    Timestamp time.Time
    Weight    float64           // measurement confidence (default 1.0)
}
```

```go
// internal/tomo/problem.go

// Problem represents a network tomography inverse problem.
type Problem struct {
    Topo          Topology
    A             *mat.Dense       // Routing matrix (m paths × n links)
    B             *mat.VecDense    // End-to-end measurements (m × 1)
    Weights       *mat.VecDense    // Per-measurement weights (m × 1), nil = uniform
    Paths         []PathMeasurement
    Links         []Link
    Quality       *MatrixQuality   // Computed on construction
}

// MatrixQuality describes the conditioning of the inverse problem.
type MatrixQuality struct {
    Rank              int       // rank(A)
    NumLinks          int       // n (columns of A)
    NumPaths          int       // m (rows of A)
    ConditionNumber   float64   // cond(A)
    IdentifiableFrac  float64   // fraction of links in column space of A
    UnidentifiableLinks []int   // link indices in null space
    CoveragePerLink   []int     // number of paths traversing each link
}

// Solution is the output of a Solver.
type Solution struct {
    X           *mat.VecDense      // Per-link estimates (n × 1)
    Confidence  *mat.VecDense      // Per-link confidence interval half-width (n × 1)
    Identifiable []bool            // Per-link: was this link identifiable?
    Residual    float64            // ||Ax - b||₂
    Method      string
    Duration    time.Duration
    Metadata    map[string]any     // Solver-specific (iterations, lambda, truncation rank, etc.)
}

// Solver is the interface all inference methods implement.
type Solver interface {
    Name() string
    Solve(p *Problem) (*Solution, error)
}
```

```go
// internal/measure/source.go

// MeasurementSource provides path measurements from any data source.
// Uses an iterator pattern for large datasets.
type MeasurementSource interface {
    Name() string
    Collect(ctx context.Context, opts CollectOpts) (MeasurementIter, error)
}

// MeasurementIter streams measurements without loading all into memory.
type MeasurementIter interface {
    Next() bool
    Measurement() PathMeasurement
    Err() error
    Close() error
}
```

### Data Pipeline

```
Raw data (traceroute/RIPE Atlas/ping)
    ↓ measure.Source.Collect() → MeasurementIter
    ↓ cache.Store() (optional local cache)
Streamed PathMeasurements
    ↓ topology.InferFromMeasurements() or topology.LoadZoo()
    ↓ topology.ResolveAliases() (analytical Kapar)
Graph (nodes = routers/ASes, edges = links)
    ↓ tomo.BuildRoutingMatrix()
    ↓ tomo.AnalyzeQuality() → MatrixQuality
Problem (A matrix + b vector + quality report)
    ↓ [if quality poor] plan.RecommendProbes() → suggest additional measurements
    ↓ solver.Solve()
Solution (per-link estimates x + confidence intervals + identifiability mask)
    ↓ format.* or tui.*
Output (JSON/CSV/DOT/TUI visualization)
```

### Routing Matrix Construction

The hardest engineering problem. The pipeline:

1. **Parse traceroutes** → extract IP-level hops per path
2. **Handle traceroute pathologies:**
   - **ECMP/load balancing:** Use Paris traceroute flow IDs to pin single path per flow. Detect ECMP via per-hop IP set divergence.
   - **MPLS tunnels:** Check for RFC 4950 ICMP extensions in scamper output. Flag opaque tunnel segments.
   - **Anonymous hops (`* * *`):** Discard paths with >30% anonymous hops. Use partial path constraints for the rest.
   - **Rate-limited ICMP:** Detect via per-hop RTT variance across multiple traces.
3. **IP alias resolution** → analytical Kapar (graph-based constraint propagation on IP-interface-router relationships). Deterministic, no active probing needed.
4. **Link identification** → consecutive hops on a path define a link
5. **Build graph** → nodes = routers (or ASes for coarse mode), edges = links
6. **Construct A** → binary matrix where A[i][j] = 1 if path i traverses link j
7. **Compute matrix quality** → rank(A), condition number, identifiable links, coverage per link. Warn prominently when system is poorly conditioned.

Two granularity modes:
- **Router-level:** full resolution, needs good traceroute data. Higher risk of alias errors.
- **AS-level** (default for real data): map IPs to ASes via BGP/RouteViews. More robust, fewer unknowns, better identifiability. Recommended unless traceroute quality is high.

### Simulation Framework

Critical for validation since ground truth doesn't exist for real networks.

```
1. Load topology:
   - Topology Zoo GraphML (10-15 representative ISP networks)
   - Synthetic: Barabási-Albert (scale-free) or Waxman (geographic)
   - [Phase 3] CAIDA ITDK for large-scale router-level topology

2. Assign ground-truth link metrics:
   - Delay: geographic distance × propagation_speed (~5 μs/km in fiber)
           + log-normal queueing noise (NOT Gaussian — queueing delay is heavy-tailed)
   - Loss: low baseline (0.1%) + random spikes on selected "congested" links

3. Generate measurement paths:
   - All-pairs shortest paths, or random subset
   - Configurable: fraction of paths to include (controls overdetermination)

4. Compute synthetic end-to-end measurements:
   - Path delay = sum of link delays on path + log-normal noise(μ, σ)
   - Use minimum of multiple RTT samples per path (standard practice)
   - Path loss = 1 - product(1 - link_loss) for each link on path

5. Run identifiability analysis on A
6. Run all solvers on (A, b)
7. Compare estimated x̂ against ground truth x (only for identifiable links)
8. Report:
   - RMSE, MAE, relative error (per-identifiable-link)
   - Rank correlation (bottleneck detection accuracy)
   - Detection rate: precision/recall for finding top-k worst links
   - Identifiable fraction and coverage metrics
```

Noise model parameters exposed as CLI flags:
- `--noise-model`: `lognormal` (default) or `gaussian`
- `--noise-scale`: noise scale parameter (default: 0.1 = 10% of true delay)
- `--congestion-links`: number of links with elevated delay (default: 2)
- `--congestion-factor`: multiplier for congested links (default: 5x)
- `--samples-per-path`: RTT samples per path, use minimum (default: 3)

### Measurement Design (`netlens plan`)

A unique feature and publishable contribution. Before collecting measurements, recommend which source-destination pairs to probe for maximum tomographic value.

```go
// internal/plan/design.go

// RecommendProbes suggests source-destination pairs that maximize
// the rank of the routing matrix A, given a measurement budget.
func RecommendProbes(topo Topology, existing *Problem, budget int) []ProbePair

// Greedy algorithm: iteratively select the probe that maximizes
// marginal rank gain (or minimizes condition number improvement).
```

CLI: `netlens plan --topology abilene --budget 20` → outputs recommended probe pairs.
With existing data: `netlens plan --existing measurements.json --budget 10` → recommends additional probes.

---

## Key Dependencies

| Package | Purpose | Stars |
|---------|---------|-------|
| `gonum.org/v1/gonum` | Linear algebra (SVD, QR, least squares, matrices) | 8.3k |
| `gonum.org/v1/gonum/graph` | Graph representation, DOT encoding | (part of gonum) |
| `james-bowman/sparse` | Sparse matrix storage (CSR/CSC/COO) | ~200 |
| `spf13/cobra` | CLI framework | 39k |
| `charmbracelet/bubbletea` | TUI framework | 30k |
| `charmbracelet/lipgloss` | TUI styling | 9k |
| `charmbracelet/bubbles` | TUI components (viewport, list, table) | 6k |
| `prometheus-community/pro-bing` | ICMP ping | 800 |
| `log/slog` | Structured logging (stdlib) | — |

**No CGo dependencies. Pure Go. Single binary.**

Note: `goccy/go-graphviz` was removed — it uses CGo bindings, breaking the single-binary promise. DOT output is generated in pure Go; users can pipe to external `dot` for rendering.

---

## Build Phases

### Phase 1: Core Engine + Simulation + CLI (Weeks 1-2)

**Goal:** Prove the math works. Validate solvers against known solutions on simulated topologies.

**Deliverables:**
- [ ] `go mod init github.com/Darkroom4364/netlens`
- [ ] Core types: Link, Path, Problem, Solution, Solver interface, Topology interface
- [ ] Routing matrix construction from topology + path list
- [ ] Identifiability analysis: rank(A), condition number, null space, identifiable links
- [ ] Matrix quality scoring and reporting
- [ ] Tikhonov regularized least squares solver (L-curve for λ selection)
- [ ] NNLS solver (Lawson-Hanson active-set, QR-based inner solve, ~300 LOC)
- [ ] Truncated SVD solver (discrepancy principle for truncation rank)
- [ ] Topology Zoo GraphML loader (10-15 representative topologies)
- [ ] Synthetic topology generators: Barabási-Albert, Waxman
- [ ] Simulation framework: log-normal noise, min-of-samples, configurable congestion
- [ ] `netlens simulate --topology abilene --method tikhonov --noise 0.1`
- [ ] `netlens benchmark --topologies all --methods all`
- [ ] Benchmark report: RMSE/MAE/detection-rate comparison table
- [ ] Table-driven tests for all solvers against hand-computed solutions
- [ ] Fuzz tests for topology loaders
- [ ] Solver stability tests: near-singular matrices, zero columns, large scale (500+ links)
- [ ] CI: GitHub Actions (lint + test + build)
- [ ] Structured logging via `log/slog`

**Validation:**
- Run benchmark on 10+ topologies (Topology Zoo + synthetic)
- Tikhonov should achieve <15% relative error on identifiable links with moderate noise
- NNLS should outperform TSVD on non-negative delay estimation
- Identifiability analysis correctly identifies unobservable links
- Cross-validate against scipy.optimize.nnls on same inputs (Python script in testdata/)

### Phase 2: Real Data + Alias Resolution (Weeks 3-5)

**Goal:** Work with real-world measurements. Solve the hard data engineering problems.

**Deliverables:**
- [ ] Traceroute parser: scamper JSON, mtr --json, paris-traceroute
- [ ] ECMP detection and path pinning
- [ ] MPLS tunnel detection (RFC 4950 ICMP extensions)
- [ ] Anonymous hop handling (threshold + partial path constraints)
- [ ] IP alias resolution: analytical Kapar (graph-based constraint propagation)
- [ ] AS-level mode: IP-to-AS mapping via BGP/RouteViews
- [ ] Topology inference from traceroutes
- [ ] RIPE Atlas adapter: REST API client with rate limiting, pagination, caching
- [ ] Local measurement cache (bbolt or file-based)
- [ ] `netlens scan --source ripe --target <measurement_id> --method tikhonov`
- [ ] `netlens scan --source traceroute --file measurements.json`
- [ ] Output formatters: JSON, CSV, DOT (pure Go generation)
- [ ] `netlens plan --topology inferred --budget 20` (measurement design)
- [ ] Regression tests: golden file tests for all parsers
- [ ] End-to-end snapshot tests for CLI output

**Validation:**
- Run on real RIPE Atlas traceroutes between 10+ probes
- Inferred delays respect speed-of-light lower bounds (~5 μs/km)
- Compare against GEANT weathermap data where available
- Report identifiable fraction and matrix quality for real data
- Measurement design recommendations improve rank(A) when followed

### Phase 3: TUI + Advanced Methods (Weeks 6-8, optional)

**Goal:** Visual interface. Additional solvers. Polish.

**Deliverables:**
- [ ] TUI: root Bubble Tea model with background data fetching and solver execution
- [ ] TUI panel: topology graph (ASCII box drawing with link annotations)
- [ ] TUI panel: heatmap (link delay/loss color-coded: green/yellow/red)
- [ ] TUI panel: results table (per-link estimates, sorted by severity)
- [ ] TUI panel: status bar (source, method, measurement count, matrix quality)
- [ ] Auto-refresh via tea.Tick (configurable interval)
- [ ] ADMM solver for L1 / compressed sensing
- [ ] Vardi EM solver (traffic matrix estimation — separate problem mode)
- [ ] Tomogravity (gravity prior + regularized LS)
- [ ] Per-link confidence intervals via bootstrap resampling
- [ ] PerfSONAR adapter
- [ ] ICMP probing mode (built-in traceroute using pro-bing)
- [ ] goreleaser config for cross-platform binaries
- [ ] Shell completions (Cobra built-in)
- [ ] README with architecture diagram + demo GIF

### Phase 4: Publication (optional)

- [ ] Extended benchmark: all solvers on Topology Zoo + synthetic + CAIDA ITDK
- [ ] Measurement design evaluation: show greedy probe selection improves accuracy
- [ ] Real-data validation against GEANT weathermaps / known IXP latencies
- [ ] Statistical rigor: confidence intervals, multiple runs, significance tests
- [ ] Write-up targeting PAM short paper or IMC poster
- [ ] Open-source benchmark dataset and reproduction scripts

---

## TUI Layout

```
┌──────────────────────────────────────────────────────────────────┐
│  netlens                method: tikhonov   source: ripe-atlas   │
├────────────────────────────┬─────────────────────────────────────┤
│  Topology        [18/24 L] │  Link Details                      │
│  (identifiable/total)      │                                     │
│                            │  Link: A → B                       │
│   [A]──4.2ms──[B]          │  Estimated delay:  4.2ms ± 0.8ms  │
│    |            |           │  Identifiable: ✓                   │
│   2.1ms       8.7ms ◀──── │  Paths through: 7/15               │
│    |            |           │  Rank: 3/24 (by delay)            │
│   [C]──1.3ms──[D]          │                                     │
│    |                        │  ── Congestion Alert ──            │
│   0.9ms                    │  Link B → D: 8.7ms (3.2σ above    │
│    |                        │  mean). Likely bottleneck.         │
│   [E]──?.?ms──[F]          │                                     │
│   (unidentifiable)         │  Matrix quality: rank 18/24        │
│                            │  Condition number: 12.4            │
├────────────────────────────┴─────────────────────────────────────┤
│  ↑↓ select  m method  s source  r refresh  p plan  b bench  q  │
└──────────────────────────────────────────────────────────────────┘
```

Unidentifiable links shown as `?.?ms`. Colors: green (<2ms), yellow (2-5ms), red (>5ms).

---

## Testing Strategy

1. **Unit tests (solvers):** Hand-computed 3×3, 5×5, and 10×10 systems with known solutions. Every solver must reproduce within ε. Test edge cases: rank-deficient A, zero columns, single-path systems.
2. **Numerical stability tests:** Near-singular matrices, ill-conditioned systems, very large/small values. Verify solvers degrade gracefully (return error or regularize) rather than producing garbage.
3. **Property tests (routing matrix):** For any topology + path set: A has correct dimensions, row sums match path length, column sums count coverage, rank ≤ min(m, n).
4. **Identifiability tests:** Known topologies with known null spaces. Verify unidentifiable links are correctly flagged.
5. **Integration tests (simulation):** Full pipeline on Topology Zoo — load, simulate, solve, verify error bounds on identifiable links only.
6. **Fuzz tests (parsers):** Fuzz all traceroute/JSON parsers for crash resistance.
7. **Regression tests:** Golden file tests for traceroute/RIPE Atlas parsing and CLI output.
8. **Benchmark regression:** Track solver accuracy and performance across releases.
9. **Cross-validation:** Python script in testdata/ that runs scipy.optimize.nnls on same inputs for comparison.

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| NNLS implementation bugs | Medium | High | Cross-validate against scipy.optimize.nnls. QR-based inner solve, never form AᵀA. |
| Routing matrix poorly conditioned | High | High | Identifiability analysis before solving. Report unidentifiable links. Recommend additional measurements via `netlens plan`. |
| IP alias resolution errors corrupt A | High | High | Analytical Kapar in Phase 2. Default to AS-level (fewer aliases). Report alias confidence. |
| Traceroute data too incomplete | Medium | Medium | Discard >30% anonymous paths. Report coverage. Suggest RIPE Atlas probes via `plan`. |
| TUI topology rendering hard | Medium | Low | Punt TUI to Phase 3. DOT export in Phase 2 for visualization. |
| gonum missing NNLS/L1 | Expected | Medium | Implement Lawson-Hanson (~300 LOC). ADMM for L1 (~50 LOC). Both well-documented. |
| No ground truth for real data | High | Medium | Speed-of-light bounds. GEANT weathermap comparison. Simulation cross-check. |
| Log-normal noise model inadequate | Low | Low | Expose noise model as flag. Validate against empirical RTT distributions from RIPE Atlas. |

---

## Success Criteria

1. `netlens benchmark` produces credible error metrics on 10+ topologies, with identifiability analysis
2. `netlens scan` produces physically plausible per-link estimates from real RIPE Atlas data (respects speed-of-light bounds)
3. `netlens plan` recommends probes that measurably improve matrix rank when followed
4. `go test ./...` passes with >80% coverage on core packages
5. All solvers cross-validated against scipy on shared test inputs
6. TUI renders readable topology with color-coded links and identifiability markers
7. Single binary, zero runtime dependencies, `go install github.com/Darkroom4364/netlens@latest` works
8. README with architecture diagram, usage examples, and benchmark results
