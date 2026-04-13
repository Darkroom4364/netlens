package tomo

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"gonum.org/v1/gonum/mat"
)

// BootstrapConfig controls bootstrap resampling for confidence intervals.
type BootstrapConfig struct {
	NumSamples int     // bootstrap iterations (default 100)
	Alpha      float64 // significance level (default 0.05 for 95% CI)
	Seed       int64   // 0 = random
}

func (c *BootstrapConfig) defaults() {
	if c.NumSamples <= 0 {
		c.NumSamples = 100
	}
	if c.Alpha <= 0 || c.Alpha >= 1 {
		c.Alpha = 0.05
	}
}

// Bootstrap runs bootstrap resampling and returns the original solution
// with Confidence field populated (half-width of CI per link).
func Bootstrap(p *Problem, solver Solver, cfg BootstrapConfig) (*Solution, error) {
	cfg.defaults()

	// Solve original problem.
	sol, err := solver.Solve(p)
	if err != nil {
		return nil, fmt.Errorf("base solve: %w", err)
	}

	m := p.NumPaths()
	n := p.NumLinks()

	// Set up RNG.
	seed := cfg.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))

	// Collect bootstrap estimates: samples[b][j] = estimate for link j in iteration b.
	samples := make([][]float64, cfg.NumSamples)

	for b := 0; b < cfg.NumSamples; b++ {
		// Resample row indices with replacement.
		indices := make([]int, m)
		for i := range indices {
			indices[i] = rng.Intn(m)
		}

		// Build resampled A and b.
		aData := make([]float64, m*n)
		bData := make([]float64, m)
		for i, idx := range indices {
			for j := 0; j < n; j++ {
				aData[i*n+j] = p.A.At(idx, j)
			}
			bData[i] = p.B.AtVec(idx)
		}

		bp := &Problem{
			A:       mat.NewDense(m, n, aData),
			B:       mat.NewVecDense(m, bData),
			Quality: AnalyzeQuality(mat.NewDense(m, n, aData)),
		}

		bSol, err := solver.Solve(bp)
		if err != nil {
			continue // skip failed bootstrap samples
		}
		samples[b] = make([]float64, n)
		for j := 0; j < n; j++ {
			samples[b][j] = bSol.X.AtVec(j)
		}
	}

	// Compute percentile-based CI per link.
	confidence := make([]float64, n)
	loQ := cfg.Alpha / 2
	hiQ := 1 - cfg.Alpha/2

	for j := 0; j < n; j++ {
		// Gather non-nil samples for this link.
		vals := make([]float64, 0, cfg.NumSamples)
		for b := 0; b < cfg.NumSamples; b++ {
			if samples[b] != nil {
				vals = append(vals, samples[b][j])
			}
		}
		if len(vals) < 2 {
			continue
		}
		sort.Float64s(vals)
		lo := percentile(vals, loQ)
		hi := percentile(vals, hiQ)
		confidence[j] = (hi - lo) / 2
	}

	sol.Confidence = mat.NewVecDense(n, confidence)
	return sol, nil
}

// percentile returns the q-th quantile (0 <= q <= 1) from sorted data using linear interpolation.
func percentile(sorted []float64, q float64) float64 {
	n := float64(len(sorted))
	idx := q * (n - 1)
	lo := int(idx)
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
