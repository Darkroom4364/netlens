package tomo

import (
	"fmt"
	"math"
	"time"

	"gonum.org/v1/gonum/mat"
)

// NNLSSolver implements Non-Negative Least Squares using the Lawson-Hanson
// active-set algorithm. Solves: min ||Ax - b||₂ subject to x ≥ 0.
//
// This is appropriate for link delay estimation since delays are non-negative.
type NNLSSolver struct {
	// MaxIter limits iterations (0 = 3*numLinks).
	MaxIter int
}

func (s *NNLSSolver) Name() string { return "nnls" }

func (s *NNLSSolver) Solve(p *Problem) (*Solution, error) {
	if p == nil || p.A == nil || p.B == nil {
		return nil, fmt.Errorf("%s: nil problem, routing matrix, or measurement vector", s.Name())
	}
	start := time.Now()
	m, n := p.A.Dims()

	maxIter := s.MaxIter
	if maxIter <= 0 {
		maxIter = 3 * n
	}

	x, iters, err := lawsonHanson(p.A, p.B, n, m, maxIter)
	if err != nil {
		return nil, fmt.Errorf("nnls: %w", err)
	}

	return newSolution(p, x, "nnls", start, map[string]any{
		"iterations": iters,
	}), nil
}

// lawsonHanson implements the Lawson-Hanson NNLS algorithm.
// Reference: Lawson & Hanson, "Solving Least Squares Problems", 1974, Chapter 23.
func lawsonHanson(A *mat.Dense, b *mat.VecDense, n, m, maxIter int) (*mat.VecDense, int, error) {
	x := mat.NewVecDense(n, nil)

	// P = passive set (indices where x > 0, unconstrained)
	// Z = zero set (indices where x = 0, constrained)
	passive := make([]bool, n) // passive[j] = true means j is in P

	At := A.T()

	// w = Aᵀ(b - Ax), the negative gradient
	w := mat.NewVecDense(n, nil)
	computeGradient(At, b, x, w, m)

	iter := 0
	for iter < maxIter {
		// Find the maximum w[j] for j in Z (zero set)
		maxW := math.Inf(-1)
		maxJ := -1
		for j := 0; j < n; j++ {
			if !passive[j] && w.AtVec(j) > maxW {
				maxW = w.AtVec(j)
				maxJ = j
			}
		}

		// If max w ≤ 0, all zero-set gradients are non-positive → optimal
		if maxW <= 1e-15 || maxJ < 0 {
			break
		}

		// Move maxJ from Z to P
		passive[maxJ] = true

		// Inner loop: solve unconstrained LS on passive set, fix negatives
		for innerIter := 0; innerIter < maxIter; innerIter++ {
			// Solve least squares on passive columns: min ||A_P * z_P - b||
			z := solvePassiveLS(A, b, passive, n, m)

			// Check if all z_P ≥ 0
			allNonNeg := true
			for j := 0; j < n; j++ {
				if passive[j] && z.AtVec(j) < -1e-15 {
					allNonNeg = false
					break
				}
			}

			if allNonNeg {
				// Accept solution
				for j := 0; j < n; j++ {
					if passive[j] {
						x.SetVec(j, z.AtVec(j))
					}
				}
				break
			}

			// Find alpha: interpolate between x and z to keep x ≥ 0
			alpha := 1.0
			moveToZero := -1
			for j := 0; j < n; j++ {
				if passive[j] && z.AtVec(j) < -1e-15 {
					ratio := x.AtVec(j) / (x.AtVec(j) - z.AtVec(j))
					if ratio < alpha {
						alpha = ratio
						moveToZero = j
					}
				}
			}

			// x = x + alpha * (z - x)
			for j := 0; j < n; j++ {
				if passive[j] {
					x.SetVec(j, x.AtVec(j)+alpha*(z.AtVec(j)-x.AtVec(j)))
				}
			}

			// Move indices with x[j] ≈ 0 from P to Z
			for j := 0; j < n; j++ {
				if passive[j] && x.AtVec(j) < 1e-15 {
					passive[j] = false
					x.SetVec(j, 0)
				}
			}
			_ = moveToZero
		}

		// Recompute gradient
		computeGradient(At, b, x, w, m)
		iter++
	}

	return x, iter, nil
}

// computeGradient computes w = Aᵀ(b - Ax). At must be the transpose of A.
func computeGradient(At mat.Matrix, b, x, w *mat.VecDense, m int) {
	// r = b - Ax
	r := mat.NewVecDense(m, nil)
	r.MulVec(At.T(), x)
	r.SubVec(b, r)
	// w = Aᵀr
	w.MulVec(At, r)
}

// solvePassiveLS solves min ||A_P * z_P - b||₂ using QR factorization.
// Returns full z vector with zeros for non-passive indices.
func solvePassiveLS(A *mat.Dense, b *mat.VecDense, passive []bool, n, m int) *mat.VecDense {
	// Count passive columns
	passiveIdx := make([]int, 0, n)
	for j := 0; j < n; j++ {
		if passive[j] {
			passiveIdx = append(passiveIdx, j)
		}
	}
	np := len(passiveIdx)
	if np == 0 {
		return mat.NewVecDense(n, nil)
	}

	// Extract A_P (m × np)
	apData := make([]float64, m*np)
	for ci, j := range passiveIdx {
		for i := 0; i < m; i++ {
			apData[i*np+ci] = A.At(i, j)
		}
	}
	ap := mat.NewDense(m, np, apData)

	// Solve via QR: min ||A_P * z_P - b||
	var qr mat.QR
	qr.Factorize(ap)

	zp := mat.NewVecDense(np, nil)
	if err := qr.SolveVecTo(zp, false, b); err != nil {
		// QR solve failed — return zeros
		return mat.NewVecDense(n, nil)
	}

	// Scatter back to full vector
	z := mat.NewVecDense(n, nil)
	for ci, j := range passiveIdx {
		z.SetVec(j, zp.AtVec(ci))
	}
	return z
}
