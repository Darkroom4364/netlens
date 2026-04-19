package tomo

import (
	"fmt"
	"math"
	"time"

	"gonum.org/v1/gonum/mat"
)

// TomogravitySolver combines a gravity-model prior with Tikhonov regularization.
// The gravity prior assigns each link a "fair share" of the end-to-end measurements
// that traverse it, then a Tikhonov step corrects the residual.
type TomogravitySolver struct {
	// Lambda is the regularization parameter for the residual correction.
	// If 0, uses GCV to select automatically.
	Lambda float64
}

func (s *TomogravitySolver) Name() string { return "tomogravity" }

func (s *TomogravitySolver) Solve(p *Problem) (*Solution, error) {
	if p == nil || p.A == nil || p.B == nil {
		return nil, fmt.Errorf("%s: nil problem, routing matrix, or measurement vector", s.Name())
	}
	start := time.Now()
	m, n := p.A.Dims()

	// Step 1: Compute gravity prior.
	// For each link j, prior[j] = sum of b[i] for paths through j, divided by
	// the average path length of those paths.
	// Precompute path lengths (number of links per path).
	pathLen := make([]float64, m)
	for i := 0; i < m; i++ {
		for k := 0; k < n; k++ {
			if p.A.At(i, k) != 0 {
				pathLen[i]++
			}
		}
		if pathLen[i] == 0 {
			return nil, fmt.Errorf("tomogravity: path %d has no links in routing matrix", i)
		}
	}

	// Gravity prior: each path distributes b[i]/pathLen[i] to its links.
	prior := mat.NewVecDense(n, nil)
	count := make([]float64, n)
	for j := 0; j < n; j++ {
		sum := 0.0
		for i := 0; i < m; i++ {
			if p.A.At(i, j) != 0 {
				sum += p.B.AtVec(i) / pathLen[i]
				count[j]++
			}
		}
		if count[j] > 0 {
			prior.SetVec(j, sum/count[j])
		}
	}

	// Step 2: Compute residual r = b - A*prior.
	Aprior := mat.NewVecDense(m, nil)
	Aprior.MulVec(p.A, prior)
	r := mat.NewVecDense(m, nil)
	r.SubVec(p.B, Aprior)

	// Step 3: Solve residual with Tikhonov via SVD (cached on Problem).
	svdPtr, ok := p.SVD()
	if !ok {
		return nil, fmt.Errorf("tomogravity: SVD factorization failed")
	}

	sv := make([]float64, min(m, n))
	svdPtr.Values(sv)

	var u, v mat.Dense
	svdPtr.UTo(&u)
	svdPtr.VTo(&v)

	lambda := s.Lambda
	if lambda <= 0 {
		lambda = selectLambdaGCV(sv, &u, r, m, n)
	}

	// Tikhonov on residual: x_resid = V * diag(f_j * (uᵀr)/σ_j) where f_j = σ_j²/(σ_j²+λ)
	xResid := mat.NewVecDense(n, nil)
	svThresh := math.Max(sv[0]*svdTolerance, 1e-300)
	for j := 0; j < len(sv); j++ {
		sj := sv[j]
		if sj < svThresh {
			continue
		}
		filter := sj * sj / (sj*sj + lambda)
		uTr := 0.0
		for i := 0; i < m; i++ {
			uTr += u.At(i, j) * r.AtVec(i)
		}
		coeff := filter * uTr / sj
		for i := 0; i < n; i++ {
			xResid.SetVec(i, xResid.AtVec(i)+coeff*v.At(i, j))
		}
	}

	// Step 4: Final solution x = prior + x_resid.
	// Clamp negative values but preserve small positive corrections
	// by scaling the residual rather than hard-clipping the sum.
	x := mat.NewVecDense(n, nil)
	x.AddVec(prior, xResid)
	for i := 0; i < n; i++ {
		if x.AtVec(i) < 0 {
			// Fall back to the prior value (clamped to zero) rather than
			// zeroing out the Tikhonov correction entirely.
			p := prior.AtVec(i)
			if p < 0 {
				p = 0
			}
			x.SetVec(i, p)
		}
	}

	return newSolution(p, x, "tomogravity", start, map[string]any{
		"lambda": lambda,
	}), nil
}
