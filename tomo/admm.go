package tomo

import (
	"context"
	"fmt"
	"math"
	"time"

	"gonum.org/v1/gonum/mat"
)

// ADMMSolver implements ADMM for L1-minimized compressed sensing:
//
//	min ||x||₁  subject to  Ax = b
//
// Useful when the link metric vector is sparse (few congested links).
type ADMMSolver struct {
	Lambda  float64 // L1 penalty (0 = auto: 0.1 * ||Aᵀb||∞)
	Rho     float64 // ADMM penalty parameter (0 = default 1.0)
	MaxIter int     // Maximum iterations (0 = default 200)
}

func (s *ADMMSolver) Name() string { return "admm" }

func (s *ADMMSolver) Solve(ctx context.Context, p *Problem) (*Solution, error) {
	if p == nil || p.A == nil || p.B == nil {
		return nil, fmt.Errorf("%s: nil problem, routing matrix, or measurement vector", s.Name())
	}
	start := time.Now()
	m, n := p.A.Dims()
	if m == 0 || n == 0 {
		return nil, fmt.Errorf("admm: empty problem (%d×%d)", m, n)
	}

	rho := s.Rho
	if rho <= 0 {
		rho = 1.0
	}
	maxIter := s.MaxIter
	if maxIter <= 0 {
		maxIter = 200
	}

	// Precompute Aᵀ, AᵀA, Aᵀb
	At := p.A.T()
	AtA := mat.NewDense(n, n, nil)
	AtA.Mul(At, p.A)

	Atb := mat.NewVecDense(n, nil)
	Atb.MulVec(At, p.B)

	// Auto-select λ: 0.1 * ||Aᵀb||∞
	lambda := s.Lambda
	if lambda <= 0 {
		maxAbsAtb := 0.0
		for i := 0; i < n; i++ {
			if v := math.Abs(Atb.AtVec(i)); v > maxAbsAtb {
				maxAbsAtb = v
			}
		}
		lambda = 0.1 * maxAbsAtb
	}

	// Build (AᵀA + ρI) and compute its Cholesky factorization.
	// If Cholesky fails (e.g., near-singular for underdetermined problems),
	// retry with larger ρ before giving up.
	originalRho := rho
	lhs := mat.NewDense(n, n, nil)
	var chol mat.Cholesky
	cholOK := false
	for attempt := 0; attempt < 5; attempt++ {
		lhs.Copy(AtA)
		for i := 0; i < n; i++ {
			lhs.Set(i, i, lhs.At(i, i)+rho)
		}
		if chol.Factorize(mat.NewSymDense(n, lhs.RawMatrix().Data)) {
			cholOK = true
			break
		}
		if attempt < 4 {
			rho *= 10 // increase penalty to improve conditioning
		}
	}
	if !cholOK {
		return nil, fmt.Errorf("admm: Cholesky factorization failed after scaling rho to %.1e; matrix may be degenerate", rho)
	}

	// If rho was bumped for Cholesky stability, rescale lambda so that
	// kappa = lambda/rho stays equivalent to the original ratio.
	if rho != originalRho {
		lambda = lambda * (rho / originalRho)
	}

	// ADMM variables
	x := mat.NewVecDense(n, nil)
	z := mat.NewVecDense(n, nil)
	u := mat.NewVecDense(n, nil)

	rhs := mat.NewVecDense(n, nil)
	zOld := mat.NewVecDense(n, nil)
	iters := 0

	for k := 0; k < maxIter; k++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		iters = k + 1

		// x-update: x = (AᵀA + ρI)⁻¹(Aᵀb + ρ(z - u))
		for i := 0; i < n; i++ {
			rhs.SetVec(i, Atb.AtVec(i)+rho*(z.AtVec(i)-u.AtVec(i)))
		}
		if err := chol.SolveVecTo(x, rhs); err != nil {
			return nil, fmt.Errorf("admm: Cholesky solve failed: %w", err)
		}

		// z-update: z = soft_threshold(x + u, λ/ρ)
		kappa := lambda / rho
		zOld.CopyVec(z)
		for i := 0; i < n; i++ {
			v := x.AtVec(i) + u.AtVec(i)
			z.SetVec(i, softThreshold(v, kappa))
		}

		// u-update: u = u + x - z
		for i := 0; i < n; i++ {
			u.SetVec(i, u.AtVec(i)+x.AtVec(i)-z.AtVec(i))
		}

		// Convergence: primal residual ||x - z||₂ and dual residual ρ||z - z_old||₂
		primalNorm := 0.0
		dualNorm := 0.0
		for i := 0; i < n; i++ {
			pd := x.AtVec(i) - z.AtVec(i)
			primalNorm += pd * pd
			dd := z.AtVec(i) - zOld.AtVec(i)
			dualNorm += dd * dd
		}
		primalNorm = math.Sqrt(primalNorm)
		dualNorm = rho * math.Sqrt(dualNorm)

		if primalNorm < 1e-6 && dualNorm < 1e-6 {
			break
		}
	}

	return newSolution(p, x, "admm", start, map[string]any{
		"iterations": iters,
		"lambda":     lambda,
		"rho":        rho,
	}), nil
}

// softThreshold computes sign(v) * max(|v| - κ, 0).
func softThreshold(v, kappa float64) float64 {
	if v > kappa {
		return v - kappa
	}
	if v < -kappa {
		return v + kappa
	}
	return 0
}
