package tomo

import (
	"context"
	"fmt"
	"math"
	"time"

	"gonum.org/v1/gonum/mat"
)

// LaplacianSolver implements graph-Laplacian-regularized least squares:
// min ||Ax - b||² + λ||Lx||² where L is the link-graph Laplacian.
type LaplacianSolver struct {
	Lambda float64 // regularization (0 = auto)
}

func (s *LaplacianSolver) Name() string { return "laplacian" }

func (s *LaplacianSolver) Solve(ctx context.Context, p *Problem) (*Solution, error) {
	if p == nil || p.A == nil || p.B == nil {
		return nil, fmt.Errorf("%s: nil problem, routing matrix, or measurement vector", s.Name())
	}
	start := time.Now()
	m, n := p.A.Dims()
	if p.Topo == nil {
		return nil, fmt.Errorf("laplacian: topology required")
	}
	L, err := buildLinkLaplacian(p.Topo, n)
	if err != nil {
		return nil, err
	}

	lambda := s.Lambda
	if lambda <= 0 {
		// Auto-select λ via Cholesky-based grid search.
		// Precompute AᵀA, Aᵀb, LᵀL once — then each λ only needs
		// a Cholesky solve instead of a full SVD on the augmented matrix.
		//
		// Note: if AᵀA + λLᵀL is singular for all candidates (e.g. heavily
		// under-determined problems where A and L share a null-space direction),
		// Cholesky will fail and bestLam stays at 1.0. The final solve on the
		// chosen λ still uses the SVD-based solveLaplacianAug which handles
		// rank deficiency, so output quality is bounded but λ may be suboptimal.
		At := p.A.T()
		AtA := mat.NewDense(n, n, nil)
		AtA.Mul(At, p.A)
		Atb := mat.NewVecDense(n, nil)
		Atb.MulVec(At, p.B)
		LtL := mat.NewDense(n, n, nil)
		LtL.Mul(L.T(), L)

		bestLam, bestCost := 1.0, math.Inf(1)
		M := mat.NewDense(n, n, nil)
		Lx := mat.NewVecDense(n, nil)
		x := mat.NewVecDense(n, nil)
		var chol mat.Cholesky
		for i := 0; i < 40; i++ {
			lam := math.Pow(10, -4+float64(i)*0.2)
			// M = AᵀA + λ·LᵀL
			M.Scale(lam, LtL)
			M.Add(M, AtA)
			if !chol.Factorize(mat.NewSymDense(n, M.RawMatrix().Data)) {
				continue
			}
			if err := chol.SolveVecTo(x, Atb); err != nil {
				continue
			}
			Lx.MulVec(L, x)
			cost := computeResidual(p.A, x, p.B) + lam*mat.Norm(Lx, 2)
			if cost < bestCost {
				bestCost, bestLam = cost, lam
			}
		}
		lambda = bestLam
	}

	x, sv, err := solveLaplacianAug(p.A, p.B, L, m, n, lambda)
	if err != nil {
		return nil, err
	}
	return newSolution(p, x, "laplacian", start, map[string]any{
		"lambda": lambda, "singular_values": sv,
	}), nil
}

// solveLaplacianAug solves min||[A; √λL]x - [b; 0]||² via SVD.
func solveLaplacianAug(A *mat.Dense, b *mat.VecDense, L *mat.Dense, m, n int, lam float64) (*mat.VecDense, []float64, error) {
	sqrtLam := math.Sqrt(lam)
	rows := m + n
	aug := mat.NewDense(rows, n, nil)
	augB := mat.NewVecDense(rows, nil)
	aug.Slice(0, m, 0, n).(*mat.Dense).Copy(A)
	for i := 0; i < m; i++ {
		augB.SetVec(i, b.AtVec(i))
	}
	scaled := mat.NewDense(n, n, nil)
	scaled.Scale(sqrtLam, L)
	aug.Slice(m, rows, 0, n).(*mat.Dense).Copy(scaled)

	var svd mat.SVD
	if !svd.Factorize(aug, mat.SVDFull) {
		return nil, nil, fmt.Errorf("laplacian: SVD failed")
	}
	sv := make([]float64, min(rows, n))
	svd.Values(sv)
	var u, v mat.Dense
	svd.UTo(&u)
	svd.VTo(&v)
	x := mat.NewVecDense(n, nil)
	svThresh := math.Max(sv[0]*svdTolerance, 1e-300)
	for j := range sv {
		if sv[j] < svThresh {
			continue
		}
		col := u.ColView(j)
		uTb := mat.Dot(col, augB)
		for i := 0; i < n; i++ {
			x.SetVec(i, x.AtVec(i)+(uTb/sv[j])*v.At(i, j))
		}
	}
	return x, sv, nil
}

// buildLinkLaplacian builds L = D - W for the link adjacency graph.
func buildLinkLaplacian(topo Topology, n int) (*mat.Dense, error) {
	links := topo.Links()
	if len(links) < n {
		return nil, fmt.Errorf("laplacian: topology has %d links but routing matrix expects %d", len(links), n)
	}
	nodeLinks := make(map[int][]int)
	for i, l := range links[:n] {
		nodeLinks[l.Src] = append(nodeLinks[l.Src], i)
		nodeLinks[l.Dst] = append(nodeLinks[l.Dst], i)
	}
	L := mat.NewDense(n, n, nil)
	deg := make([]float64, n)
	for _, lks := range nodeLinks {
		for _, a := range lks {
			for _, b := range lks {
				if a != b {
					if L.At(a, b) == 0 {
						deg[a]++
					}
					L.Set(a, b, -1)
				}
			}
		}
	}
	for i := 0; i < n; i++ {
		L.Set(i, i, deg[i])
	}
	for i := 0; i < n; i++ {
		if L.At(i, i) == 0 {
			return nil, fmt.Errorf("laplacian: link %d is isolated (no shared nodes with other links); graph not connected", i)
		}
	}
	return L, nil
}
