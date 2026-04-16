# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

netlens is a network tomography toolkit written in pure Go. It infers internal network state (per-link latency, loss, congestion) from edge measurements alone — without access to internal routers. First modern implementation since a dead R package from 2012.

## Quick Start

```bash
go build ./cmd/netlens        # Build binary
go test -race ./...           # Run all tests
make lint                     # golangci-lint
```

## Development Commands

```bash
make build        # Build binary with version ldflags
make test         # Run tests with race detector
make test-cover   # Tests + HTML coverage report
make lint         # golangci-lint
make bench        # Run benchmarks on solver packages
make fmt          # gofmt + goimports
make clean        # Remove binary and coverage files
```

## Architecture

### Layered Design

- **Frontends:** CLI (Cobra), TUI (Bubble Tea), importable Go library
- **Analysis:** Identifiability analysis, matrix quality scoring, measurement design
- **Inference engine:** Tikhonov, NNLS, Truncated SVD, ADMM, Vardi EM, Tomogravity
- **Data adapters:** RIPE Atlas, traceroute parsers (scamper/mtr/paris), PerfSONAR, ICMP, simulation

### Directory Structure

- `cmd/netlens/` — CLI entrypoint (Cobra root)
- `internal/tomo/` — Core inference engine: solvers, routing matrix, identifiability analysis
- `internal/topology/` — Network graph representation, Topology Zoo loader, synthetic generators, IP alias resolution
- `internal/measure/` — MeasurementSource interface + adapters (RIPE Atlas, traceroute, PerfSONAR, simulation)
- `internal/plan/` — Measurement design: recommend optimal probe pairs to maximize rank(A)
- `internal/bench/` — Benchmark runner: all solvers × all topologies, error metrics
- `internal/format/` — Output formatters (JSON, CSV, DOT)
- `internal/cli/` — Cobra subcommands (scan, simulate, benchmark, plan)
- `internal/tui/` — Bubble Tea TUI with topology, heatmap, results panels
- `testdata/` — Topology Zoo GraphML files and sample measurement data

### Key Patterns

**Core interface:** All solvers implement `tomo.Solver` with `Solve(p *Problem) (*Solution, error)`. Problem contains routing matrix A, measurement vector b, weights, and matrix quality. Solution contains per-link estimates, confidence intervals, and identifiability mask.

**Data pipeline:** Raw data → `MeasurementSource.Collect()` → `topology.InferFromMeasurements()` → `tomo.BuildRoutingMatrix()` → `tomo.AnalyzeQuality()` → `solver.Solve()` → output.

**Topology interface:** `tomo.Topology` is defined in the tomo package (not topology) to prevent circular imports. The topology package implements it.

**Identifiability first:** Always run identifiability analysis before solving. Report unidentifiable links (null space of A), condition number, and coverage per link. Never present estimates for unidentifiable links as if they are reliable.

**Noise model:** Use log-normal noise for simulation (queueing delay is heavy-tailed, NOT Gaussian). Use minimum of multiple RTT samples per path.

**Numerical methods:**
- NNLS: Lawson-Hanson active-set with QR-based inner solve. Never form AᵀA explicitly.
- SVD: Always truncated (TSVD), never full pseudoinverse. Threshold via discrepancy principle.
- Tikhonov: Regularization parameter λ via L-curve or GCV.
- L1/compressed sensing: ADMM, not LP reformulation.

**Granularity modes:** AS-level (default for real data, more robust) or router-level (needs good traceroute data + alias resolution).

## Key Dependencies

- `gonum.org/v1/gonum` — linear algebra (SVD, QR, matrices)
- `spf13/cobra` — CLI framework
- `charmbracelet/bubbletea` — TUI framework
- `charmbracelet/lipgloss` — TUI styling
- `prometheus-community/pro-bing` — ICMP ping

No CGo. Pure Go. Single binary.

## Conventions

- Apache 2.0 license
- All solvers must be validated against hand-computed solutions in tests
- Cross-validate NNLS against scipy.optimize.nnls (Python script in testdata/)
- DOT output is pure Go generation (no graphviz CGo bindings)
