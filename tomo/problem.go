package tomo

import (
	"sync"
	"time"

	"gonum.org/v1/gonum/mat"
)

// Problem represents a network tomography inverse problem: y = Ax + e
// where A is the routing matrix, x is per-link metrics, y is end-to-end measurements.
// A Problem must not be copied after construction (contains sync.Once).
type Problem struct {
	Topo    Topology
	A       *mat.Dense    // Routing matrix (m paths × n links)
	B       *mat.VecDense // End-to-end measurements (m × 1)
	Weights *mat.VecDense // Per-measurement weights (m × 1), nil = uniform
	Paths   []PathSpec
	Links   []Link
	Quality *MatrixQuality // Computed during construction

	svdOnce sync.Once // guards lazy SVD computation
	svdFull *mat.SVD  // cached full SVD of A
	svdOK   bool      // whether Factorize succeeded
}

// SVD returns the cached full SVD of A, computing it on first call.
// Thread-safe via sync.Once.
func (p *Problem) SVD() (*mat.SVD, bool) {
	p.svdOnce.Do(func() {
		p.svdFull = &mat.SVD{}
		p.svdOK = p.svdFull.Factorize(p.A, mat.SVDFull)
	})
	return p.svdFull, p.svdOK
}

// NumPaths returns the number of measurement paths (rows of A).
func (p *Problem) NumPaths() int {
	m, _ := p.A.Dims()
	return m
}

// NumLinks returns the number of links (columns of A).
func (p *Problem) NumLinks() int {
	_, n := p.A.Dims()
	return n
}

// MatrixQuality describes the conditioning of the inverse problem.
type MatrixQuality struct {
	Rank                int       // rank(A)
	NumLinks            int       // n (columns of A)
	NumPaths            int       // m (rows of A)
	ConditionNumber     float64   // cond(A) — ratio of largest to smallest nonzero singular value
	IdentifiableFrac    float64   // fraction of links in column space of A
	UnidentifiableLinks []int     // link indices in null space of A
	CoveragePerLink     []int     // number of paths traversing each link
	SingularValues      []float64 // all singular values (for diagnostics)
}

// IsIdentifiable returns true if the given link index has an identifiable metric.
func (q *MatrixQuality) IsIdentifiable(linkIdx int) bool {
	for _, idx := range q.UnidentifiableLinks {
		if idx == linkIdx {
			return false
		}
	}
	return true
}

// Solution is the output of a Solver.
type Solution struct {
	X            *mat.VecDense // Per-link estimates (n × 1)
	Confidence   *mat.VecDense // Per-link confidence interval half-width (n × 1), may be nil
	Identifiable []bool        // Per-link: was this link identifiable?
	Residual     float64       // ||Ax - b||₂
	Method       string
	Duration     time.Duration
	Metadata     map[string]any // Solver-specific (iterations, lambda, truncation rank, etc.)
}

// Solver is the interface all inference methods implement.
// Implementations must be safe for concurrent Solve calls;
// Solve must not mutate receiver state.
type Solver interface {
	Name() string
	Solve(p *Problem) (*Solution, error)
}

// identifiabilityMask builds a per-link boolean slice from the quality analysis.
func identifiabilityMask(q *MatrixQuality, n int) []bool {
	mask := make([]bool, n)
	if q != nil {
		for i := range mask {
			mask[i] = q.IsIdentifiable(i)
		}
	}
	return mask
}

// newSolution constructs a Solution with residual, identifiability, and timing.
func newSolution(p *Problem, x *mat.VecDense, method string, start time.Time, meta map[string]any) *Solution {
	_, n := p.A.Dims()
	return &Solution{
		X:            x,
		Identifiable: identifiabilityMask(p.Quality, n),
		Residual:     computeResidual(p.A, x, p.B),
		Method:       method,
		Duration:     time.Since(start),
		Metadata:     meta,
	}
}
