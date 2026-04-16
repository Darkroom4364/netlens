package tomo

import (
	"fmt"
	"math"
	"time"

	"gonum.org/v1/gonum/mat"
)

// TSVDSolver implements truncated SVD for network tomography.
// Truncation avoids amplifying noise through small singular values.
type TSVDSolver struct {
	// TruncationRank overrides automatic rank selection if > 0.
	TruncationRank int
}

func (s *TSVDSolver) Name() string { return "tsvd" }

func (s *TSVDSolver) Solve(p *Problem) (*Solution, error) {
	if p == nil || p.A == nil || p.B == nil {
		return nil, fmt.Errorf("%s: nil problem, routing matrix, or measurement vector", s.Name())
	}
	start := time.Now()
	m, n := p.A.Dims()

	var svd mat.SVD
	if !svd.Factorize(p.A, mat.SVDFull) {
		return nil, fmt.Errorf("tsvd: SVD factorization failed")
	}

	sv := make([]float64, min(m, n))
	svd.Values(sv)

	var u, v mat.Dense
	svd.UTo(&u)
	svd.VTo(&v)

	// Determine truncation rank
	k := s.truncationRank(sv, p.B, &u, m, n)

	// Compute x = V_k * Sigma_k^{-1} * U_k^T * b
	// where only the first k singular components are used
	x := mat.NewVecDense(n, nil)

	for j := 0; j < k; j++ {
		if sv[j] < 1e-15 {
			continue
		}
		// Compute u_j^T * b
		uTb := 0.0
		for i := 0; i < m; i++ {
			uTb += u.At(i, j) * p.B.AtVec(i)
		}
		// x += (u_j^T * b / sigma_j) * v_j
		coeff := uTb / sv[j]
		for i := 0; i < n; i++ {
			x.SetVec(i, x.AtVec(i)+coeff*v.At(i, j))
		}
	}

	return newSolution(p, x, "tsvd", start, map[string]any{
		"truncation_rank": k,
		"singular_values": sv,
	}), nil
}

// truncationRank determines k using the discrepancy principle or user override.
func (s *TSVDSolver) truncationRank(sv []float64, b *mat.VecDense, u *mat.Dense, m, n int) int {
	if s.TruncationRank > 0 {
		return min(s.TruncationRank, len(sv))
	}

	// Discrepancy principle: choose k such that the residual norm
	// is approximately equal to the noise level.
	// Estimate noise as the smallest singular values' contribution.
	// Heuristic: keep components where sigma_j > sigma_max * sqrt(eps) * max(m,n)
	maxSV := sv[0]
	threshold := maxSV * math.Sqrt(1e-15) * float64(max(m, n))

	k := 0
	for _, s := range sv {
		if s > threshold {
			k++
		}
	}
	if k == 0 {
		k = 1
	}
	return k
}

// computeResidual returns ||Ax - b||₂
func computeResidual(A *mat.Dense, x *mat.VecDense, b *mat.VecDense) float64 {
	m, _ := A.Dims()
	r := mat.NewVecDense(m, nil)
	r.MulVec(A, x)
	r.SubVec(r, b)
	return mat.Norm(r, 2)
}
