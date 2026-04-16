package tomo

import (
	"fmt"
	"math"
	"time"

	"gonum.org/v1/gonum/mat"
)

// TikhonovSolver implements Tikhonov-regularized least squares:
// min ||Ax - b||² + λ||x||²
// Solution: x = (AᵀA + λI)⁻¹ Aᵀb
type TikhonovSolver struct {
	// Lambda is the regularization parameter. If 0, uses automatic selection.
	Lambda float64
	// LambdaMethod selects the automatic method: "gcv" (default) or "lcurve".
	LambdaMethod string
}

func (s *TikhonovSolver) Name() string { return "tikhonov" }

func (s *TikhonovSolver) Solve(p *Problem) (*Solution, error) {
	if p == nil || p.A == nil || p.B == nil {
		return nil, fmt.Errorf("%s: nil problem, routing matrix, or measurement vector", s.Name())
	}
	start := time.Now()
	_, n := p.A.Dims()

	// Compute SVD for efficient Tikhonov solution
	var svd mat.SVD
	if !svd.Factorize(p.A, mat.SVDFull) {
		return nil, fmt.Errorf("tikhonov: SVD factorization failed")
	}

	m, _ := p.A.Dims()
	sv := make([]float64, min(m, n))
	svd.Values(sv)

	var u, v mat.Dense
	svd.UTo(&u)
	svd.VTo(&v)

	// Select lambda
	lambda := s.Lambda
	if lambda <= 0 {
		if s.LambdaMethod == "lcurve" {
			lambda = selectLambdaLCurve(sv, &u, p.B, m, n)
		} else {
			lambda = selectLambdaGCV(sv, &u, p.B, m, n)
		}
	}

	// Tikhonov solution via SVD:
	// x = Σ_j (σ_j / (σ_j² + λ)) * (u_j · b) * v_j
	x := mat.NewVecDense(n, nil)
	for j := 0; j < len(sv); j++ {
		sj := sv[j]
		if sj < 1e-15 {
			continue
		}
		// Filter factor: f_j = σ_j² / (σ_j² + λ)
		filter := sj * sj / (sj*sj + lambda)

		uTb := 0.0
		for i := 0; i < m; i++ {
			uTb += u.At(i, j) * p.B.AtVec(i)
		}
		coeff := filter * uTb / sj
		for i := 0; i < n; i++ {
			x.SetVec(i, x.AtVec(i)+coeff*v.At(i, j))
		}
	}

	return newSolution(p, x, "tikhonov", start, map[string]any{
		"lambda":          lambda,
		"singular_values": sv,
	}), nil
}

// selectLambdaGCV selects the regularization parameter using
// Generalized Cross-Validation (GCV).
// GCV(λ) = (1/m) * ||Ax_λ - b||² / (1 - trace(A_λ)/m)²
// where A_λ = A(AᵀA + λI)⁻¹Aᵀ is the influence matrix.
func selectLambdaGCV(sv []float64, u *mat.Dense, b *mat.VecDense, m, n int) float64 {
	// Precompute u_j · b
	k := len(sv)
	uTb := make([]float64, k)
	for j := 0; j < k; j++ {
		for i := 0; i < m; i++ {
			uTb[j] += u.At(i, j) * b.AtVec(i)
		}
	}

	// Search over log-spaced lambda values
	minLambda := sv[k-1] * sv[k-1] * 1e-6
	maxLambda := sv[0] * sv[0] * 10
	if minLambda <= 0 {
		minLambda = 1e-12
	}

	bestLambda := minLambda
	bestGCV := math.Inf(1)

	nPoints := 100
	logMin := math.Log10(minLambda)
	logMax := math.Log10(maxLambda)

	for i := 0; i < nPoints; i++ {
		logLam := logMin + (logMax-logMin)*float64(i)/float64(nPoints-1)
		lam := math.Pow(10, logLam)

		// Residual norm squared: Σ_j (λ/(σ_j²+λ))² * (u_j·b)²
		// + Σ_{j>k} (u_j·b)² (components outside SVD range)
		residSq := 0.0
		traceA := 0.0
		for j := 0; j < k; j++ {
			sj2 := sv[j] * sv[j]
			filter := sj2 / (sj2 + lam)
			residSq += (1 - filter) * (1 - filter) * uTb[j] * uTb[j]
			traceA += filter
		}

		denom := (1.0 - traceA/float64(m))
		if denom <= 1e-15 {
			continue
		}
		gcv := residSq / float64(m) / (denom * denom)

		if gcv < bestGCV {
			bestGCV = gcv
			bestLambda = lam
		}
	}

	return bestLambda
}

// selectLambdaLCurve selects λ by finding the point of maximum curvature
// on the L-curve (log-log plot of residual norm vs solution norm).
func selectLambdaLCurve(sv []float64, u *mat.Dense, b *mat.VecDense, m, n int) float64 {
	k := len(sv)
	uTb := make([]float64, k)
	for j := 0; j < k; j++ {
		for i := 0; i < m; i++ {
			uTb[j] += u.At(i, j) * b.AtVec(i)
		}
	}

	minLambda := sv[k-1] * sv[k-1] * 1e-6
	maxLambda := sv[0] * sv[0] * 10
	if minLambda <= 0 {
		minLambda = 1e-12
	}

	nPts := 100
	logMin := math.Log10(minLambda)
	logMax := math.Log10(maxLambda)

	// Compute log(rho) and log(eta) for each lambda
	logRho := make([]float64, nPts)
	logEta := make([]float64, nPts)
	lambdas := make([]float64, nPts)

	for i := 0; i < nPts; i++ {
		logLam := logMin + (logMax-logMin)*float64(i)/float64(nPts-1)
		lam := math.Pow(10, logLam)
		lambdas[i] = lam

		var residSq, solnSq float64
		for j := 0; j < k; j++ {
			sj2 := sv[j] * sv[j]
			filter := sj2 / (sj2 + lam)
			residSq += (1 - filter) * (1 - filter) * uTb[j] * uTb[j]
			coeff := filter * uTb[j] / sv[j]
			solnSq += coeff * coeff
		}
		logRho[i] = math.Log(math.Max(residSq, 1e-30)) / 2
		logEta[i] = math.Log(math.Max(solnSq, 1e-30)) / 2
	}

	// Find max curvature via discrete second derivative
	bestIdx := nPts / 2
	bestCurv := math.Inf(-1)
	for i := 1; i < nPts-1; i++ {
		xpp := logRho[i+1] - 2*logRho[i] + logRho[i-1]
		ypp := logEta[i+1] - 2*logEta[i] + logEta[i-1]
		xp := (logRho[i+1] - logRho[i-1]) / 2
		yp := (logEta[i+1] - logEta[i-1]) / 2
		denom := math.Pow(xp*xp+yp*yp, 1.5)
		if denom < 1e-30 {
			continue
		}
		curv := (xp*ypp - yp*xpp) / denom
		if curv > bestCurv {
			bestCurv = curv
			bestIdx = i
		}
	}

	return lambdas[bestIdx]
}
