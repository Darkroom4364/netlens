package tomo

import (
	"fmt"
	"math"
	"time"

	"gonum.org/v1/gonum/mat"
)

// IRL1Solver implements Iterative Reweighted L1 minimization.
// It adaptively reweights the L1 penalty so that large components are
// penalized less and near-zero components are penalized more, sharpening
// sparse recovery compared to standard ADMM.
type IRL1Solver struct {
	MaxOuterIter int     // reweighting iterations (default 5)
	MaxInnerIter int     // ADMM iterations per reweight (default 100)
	Rho          float64 // ADMM penalty parameter (default 1.0)
	Epsilon      float64 // reweighting stability param (default 0.1)
}

func (s *IRL1Solver) Name() string { return "irl1" }

func (s *IRL1Solver) Solve(p *Problem) (*Solution, error) {
	if p == nil || p.A == nil || p.B == nil {
		return nil, fmt.Errorf("%s: nil problem, routing matrix, or measurement vector", s.Name())
	}
	start := time.Now()
	m, n := p.A.Dims()
	if m == 0 || n == 0 {
		return nil, fmt.Errorf("irl1: empty problem (%d×%d)", m, n)
	}

	outerIter := s.MaxOuterIter
	if outerIter <= 0 {
		outerIter = 5
	}
	innerIter := s.MaxInnerIter
	if innerIter <= 0 {
		innerIter = 100
	}
	rho := s.Rho
	if rho <= 0 {
		rho = 1.0
	}
	eps := s.Epsilon
	if eps <= 0 {
		eps = 0.1
	}

	// Precompute Aᵀ, AᵀA, Aᵀb
	At := p.A.T()
	AtA := mat.NewDense(n, n, nil)
	AtA.Mul(At, p.A)
	Atb := mat.NewVecDense(n, nil)
	Atb.MulVec(At, p.B)

	// Auto-select λ
	maxAbs := 0.0
	for i := 0; i < n; i++ {
		if v := math.Abs(Atb.AtVec(i)); v > maxAbs {
			maxAbs = v
		}
	}
	lambda := 0.1 * maxAbs

	// Cholesky of (AᵀA + ρI) — constant across all iterations
	lhs := mat.NewDense(n, n, nil)
	lhs.Copy(AtA)
	for i := 0; i < n; i++ {
		lhs.Set(i, i, lhs.At(i, i)+rho)
	}
	var chol mat.Cholesky
	if !chol.Factorize(mat.NewSymDense(n, lhs.RawMatrix().Data)) {
		return nil, fmt.Errorf("irl1: Cholesky factorization failed")
	}

	// Weights and ADMM variables
	w := make([]float64, n)
	for i := range w {
		w[i] = 1.0
	}
	x := mat.NewVecDense(n, nil)
	z := mat.NewVecDense(n, nil)
	u := mat.NewVecDense(n, nil)
	rhsVec := mat.NewVecDense(n, nil)

	zOld := mat.NewVecDense(n, nil)
	totalIters := 0
	for outer := 0; outer < outerIter; outer++ {
		for k := 0; k < innerIter; k++ {
			totalIters++
			// x-update
			for i := 0; i < n; i++ {
				rhsVec.SetVec(i, Atb.AtVec(i)+rho*(z.AtVec(i)-u.AtVec(i)))
			}
			if err := chol.SolveVecTo(x, rhsVec); err != nil {
				return nil, fmt.Errorf("irl1: solve failed: %w", err)
			}
			// z-update: weighted soft-threshold
			zOld.CopyVec(z)
			for i := 0; i < n; i++ {
				v := x.AtVec(i) + u.AtVec(i)
				kappa := w[i] * lambda / rho
				z.SetVec(i, softThreshold(v, kappa))
			}
			// u-update
			for i := 0; i < n; i++ {
				u.SetVec(i, u.AtVec(i)+x.AtVec(i)-z.AtVec(i))
			}
			// Convergence check
			pNorm, dNorm := 0.0, 0.0
			for i := 0; i < n; i++ {
				pd := x.AtVec(i) - z.AtVec(i)
				pNorm += pd * pd
				dd := z.AtVec(i) - zOld.AtVec(i)
				dNorm += dd * dd
			}
			if math.Sqrt(pNorm) < 1e-6 && rho*math.Sqrt(dNorm) < 1e-6 {
				break
			}
		}
		// Reweight
		for i := 0; i < n; i++ {
			w[i] = 1.0 / (math.Abs(z.AtVec(i)) + eps)
		}
	}

	residual := computeResidual(p.A, x, p.B)
	identifiable := make([]bool, n)
	if p.Quality != nil {
		for i := range identifiable {
			identifiable[i] = p.Quality.IsIdentifiable(i)
		}
	}
	return &Solution{
		X: x, Identifiable: identifiable, Residual: residual,
		Method: "irl1", Duration: time.Since(start),
		Metadata: map[string]any{
			"outer_iterations": outerIter, "total_iterations": totalIters,
			"lambda": lambda, "rho": rho, "epsilon": eps,
		},
	}, nil
}
