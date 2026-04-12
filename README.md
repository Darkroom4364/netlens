# netlens

Network tomography toolkit -- infer what you can't observe.

## Overview

netlens solves the inverse problem of network tomography: given end-to-end path measurements (traceroutes, pings), it estimates internal per-link latency, loss, and congestion without access to internal routers. It builds a routing matrix from observed paths, analyzes identifiability, and applies regularized solvers to recover link-level state. The toolkit includes a simulation framework for validation against ground truth on real ISP topologies from Topology Zoo, a measurement design planner, and an interactive TUI.

## Features

- **Six solvers** for link delay estimation and traffic matrix recovery (Tikhonov, NNLS, TSVD, ADMM, Vardi EM, Tomogravity)
- **Identifiability analysis** -- rank, null space, condition number, and per-link coverage before solving
- **Real data ingest** from RIPE Atlas API and local traceroute files (scamper, mtr, paris formats)
- **Simulation engine** with log-normal noise, configurable congestion, and Topology Zoo topologies
- **Benchmark runner** comparing all solvers across all topologies with RMSE, MAE, and detection rate
- **Measurement design** -- greedy probe selection to maximize routing matrix rank
- **Interactive TUI** with topology view and link heatmap (Bubble Tea)
- **Output formats** -- table, JSON, CSV, DOT graph
- Pure Go, no CGo, single binary

## Install

```
go install github.com/Darkroom4364/netlens/cmd/netlens@latest
```

Or download a binary from [Releases](https://github.com/Darkroom4364/netlens/releases).

## Quick Start

```bash
# Simulate on the Abilene topology with 10% log-normal noise
netlens simulate -t testdata/topologies/abilene.graphml -m tikhonov --noise 0.1

# Scan real traceroutes from a local file
netlens scan --source traceroute --file traces.json -m nnls -f json

# Scan live RIPE Atlas measurements
netlens scan --source ripe --msm 12345678 -m tikhonov --cache

# Benchmark all solvers across all topologies
netlens benchmark -t testdata/topologies/ --noise 0.1

# Benchmark on synthetic topologies (Barabasi-Albert, Waxman)
netlens benchmark --synthetic --seed 42

# Plan optimal probe placement for Abilene
netlens plan -t testdata/topologies/abilene.graphml --budget 15

# Launch the interactive TUI
netlens tui -t testdata/topologies/geant2012.graphml -m nnls
```

## Commands

| Command      | Key Flags                                                       | Description                                                |
|--------------|-----------------------------------------------------------------|------------------------------------------------------------|
| `simulate`   | `-t` topology, `-m` method, `--noise`, `--congestion-links`    | Run inference on simulated topology with known ground truth |
| `scan`       | `--source`, `--msm`, `--file`, `-m` method, `-f` format        | Infer link metrics from real measurement data              |
| `benchmark`  | `-t` topologies dir, `--synthetic`, `--noise`, `--seed`         | Compare all solvers across topologies (RMSE, MAE, etc.)    |
| `plan`       | `-t` topology, `-b` budget                                      | Recommend probe pairs to maximize routing matrix rank      |
| `tui`        | `-t` topology, `-m` method                                      | Interactive TUI with topology view and heatmap             |
| `completion` | `bash`, `zsh`, `fish`                                           | Generate shell completion script                           |
| `version`    |                                                                 | Print version                                              |

## Example Output

Benchmark output (truncated -- 3 topologies, 4 solvers):

```
Topology             Solver     Nodes Links Paths Rank     Cond     RMSE      MAE  MaxRelErr Ident  Time(ms)
--------------------------------------------------------------------------------------------------------------
abilene              tsvd          12    30    66   30      4.2   0.3412   0.2198     12.34%  100%      1.20
abilene              tikhonov      12    30    66   30      4.2   0.2876   0.1934      9.87%  100%      0.85
abilene              nnls          12    30    66   30      4.2   0.2541   0.1702      8.91%  100%      2.10
abilene              vardi-em      12    30    66   30      4.2   0.4123   0.3011     15.22%  100%      3.40
sprint               tsvd          52   168   338  152     28.7   0.8234   0.5112     34.56%   90%      8.50
sprint               tikhonov      52   168   338  152     28.7   0.6891   0.4234     28.12%   90%     12.30
sprint               nnls          52   168   338  152     28.7   0.5923   0.3812     25.67%   90%     45.20
sprint               vardi-em      52   168   338  152     28.7   1.2340   0.8901     52.34%   90%     18.70
geant2012            tsvd          40   122   240  118     18.3   0.6123   0.4012     22.45%   97%      5.30
geant2012            tikhonov      40   122   240  118     18.3   0.5234   0.3412     18.91%   97%      4.80
geant2012            nnls          40   122   240  118     18.3   0.4891   0.3102     16.78%   97%     28.40
geant2012            vardi-em      40   122   240  118     18.3   0.7812   0.5623     38.12%   97%     12.10
```

## Architecture

```
+-------------------------------------------------------+
|  Frontends                                            |
|  +-- CLI     (netlens scan/simulate/benchmark/plan)   |
|  +-- TUI     (real-time topology + heatmap)           |
|  +-- Library (import as Go package)                   |
+-------------------------------------------------------+
|  Analysis layer                                       |
|  +-- Identifiability analysis (rank, null space)      |
|  +-- Matrix quality scoring (condition number, cov.)  |
|  +-- Measurement design (optimal probe selection)     |
|  +-- Anomaly detection                                |
|  +-- Simulation / benchmarking                        |
+-------------------------------------------------------+
|  Inference engine (core)                              |
|                                                       |
|  Link delay / loss estimation (Ax = b):               |
|  +-- Tikhonov regularized least squares               |
|  +-- NNLS (non-negative least squares)                |
|  +-- Truncated SVD (TSVD)                             |
|  +-- ADMM (L1 / compressed sensing)                   |
|                                                       |
|  Traffic matrix estimation:                           |
|  +-- Vardi EM                                         |
|  +-- Tomogravity                                      |
+-------------------------------------------------------+
|  Data adapters (MeasurementSource interface)          |
|  +-- RIPE Atlas API                                   |
|  +-- Traceroute parsers (scamper, mtr, paris)         |
|  +-- PerfSONAR                                        |
|  +-- ICMP ping (pro-bing)                             |
|  +-- Simulated (Topology Zoo + synthetic noise)       |
+-------------------------------------------------------+
```

## Solvers

| Solver       | Method                          | Best for                              | Non-negative |
|--------------|---------------------------------|---------------------------------------|--------------|
| `tikhonov`   | Regularized least squares       | General-purpose, smooth solutions     | No           |
| `nnls`       | Lawson-Hanson active set        | Physical quantities (delay, loss)     | Yes          |
| `tsvd`       | Truncated SVD                   | Ill-conditioned matrices, fast        | No           |
| `admm`       | ADMM with L1 penalty            | Sparse anomalies, compressed sensing  | Yes          |
| `vardi`      | Vardi EM algorithm              | Traffic matrix estimation             | Yes          |
| `tomogravity`| Gravity model + regularization  | Traffic matrix with prior information | Yes          |

## Data Sources

| Source        | Description                                 | Mode      |
|---------------|---------------------------------------------|-----------|
| RIPE Atlas    | Public traceroute/ping measurement platform | Real-time |
| Traceroute    | Local scamper, mtr, or paris-traceroute JSON | File      |
| PerfSONAR     | Research network measurement infrastructure | Real-time |
| ICMP ping     | Direct probing via pro-bing library         | Real-time |
| Simulation    | Topology Zoo graphs with synthetic noise    | Synthetic |

## Documentation

Detailed documentation is available in the `docs/` directory:

- [Solvers](docs/solvers.md) -- solver algorithms, parameters, and tuning
- [Data Sources](docs/data-sources.md) -- adapter configuration and authentication
- [Measurement Design](docs/measurement-design.md) -- probe selection and rank optimization
- [TUI](docs/tui.md) -- interactive mode keybindings and panels
- [API](docs/api.md) -- using netlens as a Go library

## License

Apache 2.0

## Contributing

Issues and PRs welcome.
