<p align="center">
  <img src="assets/netlens-logo.svg" alt="netlens" width="150" height="150" />
</p>

<h1 align="center">netlens</h1>

<p align="center">
  <em>Infer what you can't observe.</em>
</p>

<!-- Row 1 — primary metrics -->
<p align="center">
  <img alt="Solvers" src="https://img.shields.io/badge/solvers-9-e63946?style=flat-square&labelColor=2b2d42">
  <img alt="Tests" src="https://img.shields.io/badge/tests-340+-e63946?style=flat-square&labelColor=2b2d42">
  <img alt="Papers" src="https://img.shields.io/badge/cited%20papers-49-e63946?style=flat-square&labelColor=2b2d42">
</p>

<!-- Row 2 — meta -->
<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/go-1.23+-1d3557?style=flat-square&labelColor=2b2d42&logo=go&logoColor=white">
  <img alt="License" src="https://img.shields.io/badge/license-Apache%202.0-1d3557?style=flat-square&labelColor=2b2d42">
  <a href="https://github.com/Darkroom4364/netlens/actions"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/Darkroom4364/netlens/ci.yml?style=flat-square&labelColor=2b2d42&label=build"></a>
  <img alt="Deps" src="https://img.shields.io/badge/CGo-zero-457b9d?style=flat-square&labelColor=2b2d42">
</p>

---

**netlens** is a network tomography toolkit that infers internal network state, per-link latency, loss, and congestion, from edge measurements alone, without access to internal routers. Feed it traceroutes. See the invisible.

The math has existed in textbooks for 20 years. The only prior implementation was a dead R package from 2012. netlens is the first modern, usable network tomography toolkit — 9 solvers, real data pipelines, and a benchmark framework, all in a single Go binary.

## Install

```bash
go install github.com/Darkroom4364/netlens/cmd/netlens@latest
```

Or download a binary from [Releases](https://github.com/Darkroom4364/netlens/releases).

## Quick Start

```bash
# Simulate on an ISP topology with 10% log-normal noise
netlens simulate -t testdata/topologies/abilene.graphml -m nnls --noise 0.1

# Benchmark all 8 solvers across 10 real ISP topologies
netlens benchmark -t testdata/topologies --noise 0.1

# Scan real RIPE Atlas traceroutes (measurement 5001 is public, free)
export RIPE_ATLAS_API_KEY=your-key
netlens scan --source ripe --msm 5001

# Scan a local traceroute file
netlens scan --source traceroute --file measurements.json --format json

# Recommend optimal measurement probes
netlens plan -t testdata/topologies/geant2012.graphml --budget 20
```

## Benchmark

All solvers on real ISP topologies from [Topology Zoo](http://www.topology-zoo.org/) with 10% log-normal noise:

```
Topology         Solver      Nodes Links Paths Rank  Cond    RMSE     MAE   Ident
─────────────────────────────────────────────────────────────────────────────────
abilene          nnls           11    14    55   14   4.1  11.580   7.577   100%
abilene          vardi-em       11    14    55   14   4.1  10.112   6.429   100%
abilene          laplacian      11    14    55   14   4.1  11.614   7.588   100%
geant2012        nnls           40    61   780   61  14.4   5.301   3.752   100%
geant2012        vardi-em       40    61   780   61  14.4   4.816   3.438   100%
geant2012        laplacian      40    61   780   61  14.4   5.301   3.752   100%
dfn              nnls           58    87  1653   87  16.0   6.706   4.626   100%
dfn              vardi-em       58    87  1653   87  16.0   6.336   4.177   100%
dfn              laplacian      58    87  1653   87  16.0   6.700   4.622   100%
```

Vardi EM consistently achieves lowest RMSE. NNLS enforces non-negative delays (physically correct). Laplacian encourages smooth solutions across adjacent links.

## Solvers

| Solver | Method | Non-negative | Best for |
|--------|--------|:------------:|----------|
| **Tikhonov** | L2 regularized LS | No | General-purpose, smooth solutions |
| **NNLS** | Lawson-Hanson active-set | Yes | Real data (delays can't be negative) |
| **TSVD** | Truncated SVD | No | Fast baseline |
| **ADMM** | L1 compressed sensing | No | Sparse congestion (few bad links) |
| **IRL1** | Iterative reweighted L1 | No | Sharper sparse recovery than ADMM |
| **Vardi EM** | Expectation-maximization | Yes | Lowest RMSE on most topologies |
| **Tomogravity** | Gravity prior + Tikhonov | Yes | Traffic matrix estimation |
| **Laplacian** | Graph Laplacian prior | No | Topology-aware smoothness |

Plus **Bootstrap CI** and **Conformal Prediction** for uncertainty quantification.

## Data Sources

| Source | Type | Auth |
|--------|------|------|
| **RIPE Atlas** | Traceroute/ping via REST API | API key |
| **PerfSONAR** | Latency timeseries via esmond | None |
| **Traceroute files** | scamper JSON, mtr JSON, RIPE Atlas JSON | — |
| **ICMP probing** | Built-in traceroute via pro-bing | — |
| **Simulation** | Synthetic measurements on Topology Zoo/random graphs | — |

## Architecture

```
┌───────────────────────────────────────────────────────┐
│  Frontends                                            │
│  ├── CLI     (scan, simulate, benchmark, plan)        │
│  ├── TUI     (build tag: tui)                         │
│  └── Library (import as Go package)                   │
├───────────────────────────────────────────────────────┤
│  Analysis                                             │
│  ├── Identifiability (rank, null space, cond. number) │
│  ├── Matrix quality scoring                           │
│  ├── Measurement design (optimal probe selection)     │
│  ├── Speed-of-light delay validation                  │
│  └── ECMP detection · IP alias resolution             │
├───────────────────────────────────────────────────────┤
│  Inference engine — 9 solvers                         │
│  ├── Tikhonov (L-curve / GCV) · NNLS · TSVD · ADMM    │
│  ├── IRL1 · Vardi EM · Tomogravity · Laplacian        │
│  ├── Bootstrap CI · Conformal Prediction              │
│  └── Randomized SVD for large-scale matrices          │
├───────────────────────────────────────────────────────┤
│  Data adapters                                        │
│  ├── RIPE Atlas · PerfSONAR · scamper · mtr           │
│  ├── ICMP probing (build tag: probing)                │
│  └── Simulation (Topology Zoo · Barabási-Albert ·     │
│      Waxman · configurable noise models)              │
└───────────────────────────────────────────────────────┘
```

## Commands

| Command | Description |
|---------|-------------|
| `simulate` | Inference on simulated topology with synthetic noise |
| `scan` | Infer link metrics from real measurements |
| `benchmark` | Compare all solvers across topologies |
| `plan` | Recommend optimal probe pairs to maximize identifiability |
| `tui` | Interactive terminal UI *(requires `-tags tui`)* |
| `completion` | Generate shell completions (bash/zsh/fish) |

## Documentation

- [Solvers](docs/solvers.md) — algorithms, parameters, tuning
- [Data Sources](docs/data-sources.md) — adapter configuration
- [Measurement Design](docs/measurement-design.md) — probe selection
- [TUI](docs/tui.md) — interactive mode
- [API](docs/api.md) — using netlens as a Go library
- [References](docs/references.md) — 49 cited academic papers

## Build Tags

```bash
go build ./cmd/netlens                        # Core CLI (no TUI, no ICMP)
go build -tags tui ./cmd/netlens              # With TUI
go build -tags probing ./cmd/netlens          # With ICMP probing
go build -tags "tui probing" ./cmd/netlens    # Everything
```

## License

Apache 2.0
