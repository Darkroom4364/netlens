package tomo

import (
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

func (s *LaplacianSolver) Solve(p *Problem) (*Solution, error) {
	if p == nil || p.A == nil || p.B == nil {
		return nil, fmt.Errorf("%s: nil problem, routing matrix, or measurement vector", s.Name())
	}
	start := time.Now()
	m, n := p.A.Dims()
	if p.Topo == nil {
		return nil, fmt.Errorf("laplacian: topology required")
	}
	L := buildLinkLaplacian(p.Topo, n)

	lambda := s.Lambda
	if lambda <= 0 {
		// Auto-select: try log-spaced values, pick lowest combined cost
		bestLam, bestCost := 1.0, math.Inf(1)
		for i := 0; i < 40; i++ {
			lam := math.Pow(10, -4+float64(i)*0.2)
			x, _, err := solveLaplacianAug(p.A, p.B, L, m, n, lam)
			if err != nil {
				continue
			}
			Lx := mat.NewVecDense(n, nil)
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
	identifiable := make([]bool, n)
	if p.Quality != nil {
		for i := range identifiable {
			identifiable[i] = p.Quality.IsIdentifiable(i)
		}
	}
	return &Solution{
		X: x, Identifiable: identifiable, Residual: computeResidual(p.A, x, p.B),
		Method: "laplacian", Duration: time.Since(start),
		Metadata: map[string]any{"lambda": lambda, "singular_values": sv},
	}, nil
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
	for j := range sv {
		if sv[j] < 1e-15 {
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
func buildLinkLaplacian(topo Topology, n int) *mat.Dense {
	nodeLinks := make(map[int][]int)
	for i, l := range topo.Links()[:n] {
		nodeLinks[l.Src] = append(nodeLinks[l.Src], i)
		nodeLinks[l.Dst] = append(nodeLinks[l.Dst], i)
	}
	L := mat.NewDense(n, n, nil)
	for _, lks := range nodeLinks {
		for _, a := range lks {
			for _, b := range lks {
				if a != b {
					L.Set(a, b, -1)
				}
			}
		}
	}
	for i := 0; i < n; i++ {
		deg := 0.0
		for j := 0; j < n; j++ { deg -= L.At(i, j) }
		L.Set(i, i, deg)
	}
	return L
}
