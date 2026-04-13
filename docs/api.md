# netlens Go Library API Reference

```go
import (
    "github.com/Darkroom4364/netlens/tomo"
    "github.com/Darkroom4364/netlens/topology"
)
```

## Quick Example

```go
package main

import (
    "fmt"
    "os"

    "github.com/Darkroom4364/netlens/internal/format"
    "github.com/Darkroom4364/netlens/tomo"
    "github.com/Darkroom4364/netlens/topology"
)

func main() {
    // 1. Load topology from GraphML
    topo, _ := topology.LoadGraphML("testdata/Abilene.graphml")

    // 2. Build problem with synthetic ground truth (ms per link)
    groundTruth := make([]float64, topo.NumLinks())
    for i := range groundTruth {
        groundTruth[i] = float64(i+1) * 2.0
    }
    prob, _ := tomo.BuildProblemFromTopology(topo, groundTruth)

    // 3. Check identifiability
    fmt.Printf("rank %d/%d links, identifiable %.0f%%\n",
        prob.Quality.Rank, prob.Quality.NumLinks,
        prob.Quality.IdentifiableFrac*100)

    // 4. Solve
    solver := &tomo.NNLSSolver{}
    sol, _ := solver.Solve(prob)

    // 5. Output as JSON
    f := format.Get("json")
    f.Format(os.Stdout, prob, sol)
}
```

## Core Types (`internal/tomo`)

### Topology (interface)

```go
type Topology interface {
    NumNodes() int
    NumLinks() int
    Links()    []Link
    Nodes()    []Node
    Neighbors(nodeID int) []int
    ShortestPath(src, dst int) ([]int, bool)
    AllPairsShortestPaths() []PathSpec
}
```

Implemented by `topology.Graph`. Defined in `tomo` to avoid circular imports.

### Node, Link

```go
type Node struct {
    ID        int
    Label     string
    Latitude  float64
    Longitude float64
}

type Link struct {
    ID    int
    Src   int
    Dst   int
    SrcIP string // optional
    DstIP string // optional
}
```

### PathSpec, PathMeasurement, Hop

```go
type PathSpec struct {
    Src     int   // source node ID
    Dst     int   // destination node ID
    LinkIDs []int // ordered link indices traversed
}

type PathMeasurement struct {
    Src       string
    Dst       string
    Hops      []Hop
    RTTs      []time.Duration // multiple samples per path
    Loss      float64         // 0.0-1.0
    Timestamp time.Time
    Weight    float64         // measurement confidence (default 1.0)
}

type Hop struct {
    IP        string
    RTT       time.Duration
    TTL       int
    Anonymous bool // true if * * *
    MPLS      bool // RFC 4950 ICMP extension
}
```

`PathMeasurement.MinRTT()` returns the minimum RTT across samples (best approximation of propagation delay).

## Problem Construction

### BuildProblem

```go
func BuildProblem(topo Topology, paths []PathSpec, measurements []float64) (*Problem, error)
```

Core constructor. Builds routing matrix **A** (m paths x n links) and measurement vector **b**. Runs identifiability analysis automatically.

### BuildProblemFromTopology

```go
func BuildProblemFromTopology(topo Topology, groundTruth []float64) (*Problem, error)
```

Uses `AllPairsShortestPaths()` for routing and computes synthetic measurements **b = Ax**. Useful for simulation and benchmarking.

### BuildProblemFromMeasurements

```go
func BuildProblemFromMeasurements(topo Topology, pathMeasurements []PathMeasurement, pathSpecs []PathSpec) (*Problem, error)
```

Converts real `PathMeasurement` data to a `Problem`. Uses `MinRTT()` (in ms) as the end-to-end observation. Populates per-measurement weights.

## MatrixQuality and Identifiability

Every `Problem` has a `Quality *MatrixQuality` computed via SVD:

```go
type MatrixQuality struct {
    Rank                int
    NumLinks            int
    NumPaths            int
    ConditionNumber     float64
    IdentifiableFrac    float64   // fraction of links in column space
    UnidentifiableLinks []int     // link indices in null space
    CoveragePerLink     []int     // paths per link
    SingularValues      []float64
}
```

`Quality.IsIdentifiable(linkIdx)` checks a single link. Always inspect identifiability before trusting solver output -- estimates for null-space links are unreliable.

## Solver Interface

```go
type Solver interface {
    Name() string
    Solve(p *Problem) (*Solution, error)
}

type Solution struct {
    X            *mat.VecDense // per-link estimates (n x 1)
    Confidence   *mat.VecDense // confidence interval half-widths, may be nil
    Identifiable []bool
    Residual     float64       // ||Ax - b||_2
    Method       string
    Duration     time.Duration
    Metadata     map[string]any
}
```

### Available Solvers

| Solver | Constructor | Use case |
|--------|-------------|----------|
| `NNLSSolver` | `&tomo.NNLSSolver{}` | Delay estimation (non-negative constraint) |
| `TikhonovSolver` | `&tomo.TikhonovSolver{Lambda: 0.1}` | Ill-conditioned problems. Lambda=0 auto-selects via GCV |
| `TSVDSolver` | `&tomo.TSVDSolver{}` | Noise suppression via singular value truncation |
| `ADMMSolver` | `&tomo.ADMMSolver{}` | Sparse solutions (few congested links), L1 minimization |
| `VardiEMSolver` | `&tomo.VardiEMSolver{MaxIter: 500}` | Classic EM, non-negative, no matrix inversion |
| `TomogravitySolver` | `&tomo.TomogravitySolver{}` | Gravity-model prior + Tikhonov correction |

## Topology (`internal/topology`)

### Loading GraphML (Topology Zoo)

```go
topo, err := topology.LoadGraphML("path/to/file.graphml")
topo, err := topology.ParseGraphML(xmlBytes)
```

### Creating Programmatically

```go
g := topology.New()
g.AddNode(tomo.Node{ID: 0, Label: "A"})
g.AddNode(tomo.Node{ID: 1, Label: "B"})
g.AddNode(tomo.Node{ID: 2, Label: "C"})
g.AddLink(0, 1) // returns link index
g.AddLink(1, 2)
```

### Synthetic Generators

```go
// Scale-free (Barabasi-Albert): n nodes, m edges per new node
g := topology.BarabasiAlbert(50, 3, seed)

// Random geometric (Waxman): n nodes, alpha/beta control density
g := topology.Waxman(50, 0.4, 0.1, seed)
```

## Topology Inference from Traceroutes

```go
func InferFromMeasurements(
    measurements []tomo.PathMeasurement,
    opts InferOpts,
) (*Graph, []tomo.PathSpec, []int, error)
```

Builds a topology graph from traceroute hops. Each unique IP becomes a node, consecutive hops define links. Anonymous hops are skipped, MPLS hops are hidden.

Returns: inferred graph, one `PathSpec` per accepted measurement, indices of accepted measurements, error.

```go
type InferOpts struct {
    MaxAnonymousFrac float64 // discard paths above this (default 0.3)
    ASLevel          bool    // AS-level granularity
    AliasResolution  bool    // merge router interfaces (Kapar)
    ECMPDetection    bool    // deduplicate ECMP paths
}
```

### Full Pipeline Example

```go
import "github.com/Darkroom4364/netlens/internal/measure"

// Parse traceroute data (scamper, RIPE Atlas, or mtr)
measurements, _ := measure.ParseScamperFile("traces.json")
// or: measure.ParseRIPEAtlasTraceroute(jsonBytes)
// or: measure.ParseMTRJSON(jsonBytes)

// Infer topology
topo, pathSpecs, accepted, _ := topology.InferFromMeasurements(
    measurements, topology.InferOpts{ECMPDetection: true},
)

// Keep only accepted measurements
var kept []tomo.PathMeasurement
for _, i := range accepted {
    kept = append(kept, measurements[i])
}

// Build and solve
prob, _ := tomo.BuildProblemFromMeasurements(topo, kept, pathSpecs)
sol, _ := (&tomo.NNLSSolver{}).Solve(prob)
```

## Output Formatting (`internal/format`)

```go
type Formatter interface {
    Format(w io.Writer, p *tomo.Problem, s *tomo.Solution) error
}

f := format.Get("json") // or "csv", "dot"
f.Format(os.Stdout, prob, sol)
```

- **json** -- structured per-link results with estimates, identifiability, and metadata
- **csv** -- tabular link-level output
- **dot** -- Graphviz DOT graph with link metrics as edge labels
