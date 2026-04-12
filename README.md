# netlens

Network tomography toolkit — infer per-link latency, loss, and congestion from edge measurements alone.

## What it does

netlens solves the inverse problem of network tomography: given end-to-end path measurements (traceroutes, pings), it estimates internal per-link metrics without access to internal routers. It builds a routing matrix from observed paths, analyzes identifiability, and applies regularized solvers (Tikhonov, NNLS, TSVD, ADMM) to recover link-level state. Includes a simulation framework for validation against ground truth on real ISP topologies from Topology Zoo.

## Install

```
go install github.com/Darkroom4364/netlens/cmd/netlens@latest
```

Or download a binary from [Releases](https://github.com/Darkroom4364/netlens/releases).

## Quick Start

```bash
# Simulate on an ISP topology with log-normal noise
netlens simulate --topology abilene --method tikhonov --noise 0.1

# Scan real measurements from RIPE Atlas
netlens scan --source ripe --target <measurement_id> --method nnls

# Benchmark all solvers across all topologies
netlens benchmark --topologies all --methods all
```

## Commands

| Command     | Description                                              |
|-------------|----------------------------------------------------------|
| `simulate`  | Run inference on simulated topology with synthetic noise |
| `scan`      | Infer link metrics from real measurement data            |
| `benchmark` | Compare all solvers across topologies (RMSE, MAE, etc.) |
| `plan`      | Recommend optimal probe pairs to maximize rank(A)        |
| `version`   | Print version                                            |

## Architecture

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
│  │  └── ADMM (L1 / compressed sensing)                │
│  │                                                    │
│  │  Traffic matrix estimation:                        │
│  │  ├── Vardi EM                                      │
│  │  └── Tomogravity                                   │
│  │                                                    │
├───────────────────────────────────────────────────────┤
│  Data adapters (MeasurementSource interface)          │
│  ├── RIPE Atlas API                                   │
│  ├── Traceroute parsers (scamper, mtr, paris)         │
│  ├── PerfSONAR                                        │
│  ├── ICMP ping (pro-bing)                             │
│  └── Simulated (Topology Zoo + synthetic noise)       │
└───────────────────────────────────────────────────────┘
```

## License

Apache 2.0
