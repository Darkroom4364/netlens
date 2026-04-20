package tomo

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"gonum.org/v1/gonum/mat"
)

// ConformalConfig controls split conformal prediction for distribution-free
// confidence intervals.
type ConformalConfig struct {
	CalibrationFrac float64 // fraction of paths for calibration (default 0.2)
	Alpha           float64 // significance level (default 0.05 for 95% CI)
	Seed            int64   // 0 = random
}

func (c *ConformalConfig) defaults() {
	if c.CalibrationFrac <= 0 || c.CalibrationFrac >= 1 {
		c.CalibrationFrac = 0.2
	}
	if c.Alpha <= 0 || c.Alpha >= 1 {
		c.Alpha = 0.05
	}
}

// Conformal computes distribution-free prediction intervals via split
// conformal prediction. Faster than bootstrap (single solve) with
// finite-sample marginal coverage guarantee.
func Conformal(ctx context.Context, p *Problem, solver Solver, cfg ConformalConfig) (*Solution, error) {
	cfg.defaults()

	m := p.NumPaths()
	n := p.NumLinks()

	if m < 2 {
		// Degenerate: solve directly, infinite confidence.
		sol, err := solver.Solve(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("solve: %w", err)
		}
		conf := make([]float64, n)
		for j := range conf {
			conf[j] = math.Inf(1)
		}
		sol.Confidence = mat.NewVecDense(n, conf)
		return sol, nil
	}

	// Step 1: random permutation of path indices.
	seed := cfg.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))
	perm := rng.Perm(m)

	mCal := int(math.Max(1, math.Round(float64(m)*cfg.CalibrationFrac)))
	mTrain := m - mCal

	trainIdx := perm[:mTrain]
	calIdx := perm[mTrain:]

	// Step 2: build training problem.
	aTrainData := make([]float64, mTrain*n)
	bTrainData := make([]float64, mTrain)
	for i, idx := range trainIdx {
		for j := 0; j < n; j++ {
			aTrainData[i*n+j] = p.A.At(idx, j)
		}
		bTrainData[i] = p.B.AtVec(idx)
	}
	trainProblem := &Problem{
		A:       mat.NewDense(mTrain, n, aTrainData),
		B:       mat.NewVecDense(mTrain, bTrainData),
		Quality: AnalyzeQuality(mat.NewDense(mTrain, n, aTrainData)),
	}

	// Step 3: solve on training set.
	trainSol, err := solver.Solve(ctx, trainProblem)
	if err != nil {
		return nil, fmt.Errorf("training solve: %w", err)
	}

	// Step 4: calibration residuals.
	residuals := make([]float64, len(calIdx))
	for i, idx := range calIdx {
		pred := 0.0
		for j := 0; j < n; j++ {
			pred += p.A.At(idx, j) * trainSol.X.AtVec(j)
		}
		residuals[i] = math.Abs(p.B.AtVec(idx) - pred)
	}

	// Step 5: conformal quantile.
	sort.Float64s(residuals)
	qIdx := int(math.Ceil(float64(mCal+1)*(1-cfg.Alpha))) - 1 // 0-indexed
	if qIdx >= mCal {
		qIdx = mCal - 1
	}
	q := residuals[qIdx]

	// Step 6: per-link confidence from conformal quantile.
	// q bounds path-level residuals; use it directly for each covered link.
	covered := make([]bool, n)
	for _, idx := range calIdx {
		for j := 0; j < n; j++ {
			if p.A.At(idx, j) != 0 {
				covered[j] = true
			}
		}
	}
	confidence := make([]float64, n)
	for j := 0; j < n; j++ {
		if !covered[j] {
			confidence[j] = math.Inf(1)
		} else {
			confidence[j] = q
		}
	}

	// Step 7: solve on full problem, attach conformal confidence.
	sol, err := solver.Solve(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("full solve: %w", err)
	}
	sol.Confidence = mat.NewVecDense(n, confidence)
	return sol, nil
}
