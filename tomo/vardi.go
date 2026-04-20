package tomo

import (
	"context"
	"fmt"
	"math"
	"time"

	"gonum.org/v1/gonum/mat"
)

// VardiEMSolver implements the Vardi (1996) EM algorithm for non-negative
// link metric estimation. Solves y = Ax by iteratively distributing path
// measurements to links proportionally to current estimates.
type VardiEMSolver struct {
	MaxIter   int     // 0 = default 500
	Tolerance float64 // 0 = default 1e-6
}

func (s *VardiEMSolver) Name() string { return "vardi-em" }

func (s *VardiEMSolver) Solve(ctx context.Context, p *Problem) (*Solution, error) {
	if p == nil || p.A == nil || p.B == nil {
		return nil, fmt.Errorf("%s: nil problem, routing matrix, or measurement vector", s.Name())
	}
	start := time.Now()
	m, n := p.A.Dims()

	if m == 0 || n == 0 {
		return nil, fmt.Errorf("vardi-em: empty problem (%d×%d)", m, n)
	}

	maxIter := s.MaxIter
	if maxIter <= 0 {
		maxIter = 500
	}
	tol := s.Tolerance
	if tol <= 0 {
		tol = 1e-6
	}

	// Precompute which links each path uses and vice versa.
	// pathLinks[i] = columns j where A(i,j) > 0
	// linkPaths[j] = rows i where A(i,j) > 0
	pathLinks := make([][]int, m)
	linkPaths := make([][]int, n)
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			if p.A.At(i, j) > 0 {
				pathLinks[i] = append(pathLinks[i], j)
				linkPaths[j] = append(linkPaths[j], i)
			}
		}
	}

	// Reject degenerate input where too few paths have links.
	var emptyPaths int
	for i := 0; i < m; i++ {
		if len(pathLinks[i]) == 0 {
			emptyPaths++
		}
	}
	if m-emptyPaths < 2 {
		return nil, fmt.Errorf("vardi-em: only %d usable paths (need ≥2); %d paths had no links", m-emptyPaths, emptyPaths)
	}

	// Initialize x_j = 1.0 (uniform positive)
	x := make([]float64, n)
	for j := range x {
		x[j] = 1.0
	}

	xNew := make([]float64, n)
	count := make([]float64, n)

	var skippedZeroDenom int
	var iter int
	for iter = 0; iter < maxIter; iter++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		for j := range xNew {
			xNew[j] = 0
			count[j] = 0
		}
		skippedZeroDenom = 0

		// E-step: for each path i, distribute b_i to links proportionally
		for i := 0; i < m; i++ {
			links := pathLinks[i]
			if len(links) == 0 {
				continue
			}
			bi := p.B.AtVec(i)
			// sum of x_j for links on this path (weighted by A)
			var denom float64
			for _, j := range links {
				denom += p.A.At(i, j) * x[j]
			}
			if denom <= 0 {
				skippedZeroDenom++
				continue
			}
			for _, j := range links {
				w := p.A.At(i, j) * x[j] / denom
				xNew[j] += w * bi
				count[j] += p.A.At(i, j)
			}
		}

		// M-step: average contributions
		for j := 0; j < n; j++ {
			if count[j] > 0 {
				xNew[j] /= count[j]
			}
		}

		// Check convergence: max relative change
		var maxRel float64
		for j := 0; j < n; j++ {
			if xNew[j] > 0 {
				rel := math.Abs(xNew[j]-x[j]) / xNew[j]
				if rel > maxRel {
					maxRel = rel
				}
			}
		}

		copy(x, xNew)
		if maxRel < tol {
			iter++
			break
		}
	}

	xVec := mat.NewVecDense(n, x)
	return newSolution(p, xVec, "vardi-em", start, map[string]any{
		"iterations":         iter,
		"tolerance":          tol,
		"skipped_empty":      emptyPaths,
		"skipped_zero_denom": skippedZeroDenom,
	}), nil
}
