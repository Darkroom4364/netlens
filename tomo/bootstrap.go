package tomo

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"time"

	"golang.org/x/sync/errgroup"
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

// bootstrapBuf holds pre-allocated buffers reused across bootstrap iterations.
type bootstrapBuf struct {
	indices []int
	aData   []float64
	bData   []float64
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

	// Pre-generate deterministic per-sample seeds so results are
	// reproducible regardless of goroutine scheduling order.
	seeds := make([]int64, cfg.NumSamples)
	for b := range seeds {
		seeds[b] = rng.Int63()
	}

	// Per-worker buffers, one per concurrent goroutine.
	numWorkers := runtime.GOMAXPROCS(0)
	if numWorkers > cfg.NumSamples {
		numWorkers = cfg.NumSamples
	}
	bufs := make([]bootstrapBuf, numWorkers)
	for i := range bufs {
		bufs[i] = bootstrapBuf{
			indices: make([]int, m),
			aData:   make([]float64, m*n),
			bData:   make([]float64, m),
		}
	}

	// Slot channel ensures each goroutine gets its own buffer.
	slots := make(chan int, numWorkers)
	for i := 0; i < numWorkers; i++ {
		slots <- i
	}

	// Collect bootstrap estimates: samples[b][j] = estimate for link j in iteration b.
	samples := make([][]float64, cfg.NumSamples)

	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(numWorkers)

	for b := 0; b < cfg.NumSamples; b++ {
		b := b // capture per-iteration value for goroutine closure
		g.Go(func() error {
			wid := <-slots
			defer func() { slots <- wid }()
			buf := &bufs[wid]

			localRng := rand.New(rand.NewSource(seeds[b]))

			// Resample row indices with replacement.
			for i := range buf.indices {
				buf.indices[i] = localRng.Intn(m)
			}

			// Build resampled A and b into pre-allocated buffers.
			for i, idx := range buf.indices {
				for j := 0; j < n; j++ {
					buf.aData[i*n+j] = p.A.At(idx, j)
				}
				buf.bData[i] = p.B.AtVec(idx)
			}

			// buf.aData/bData are aliased into the matrices below; safe because
			// the goroutine holds the slot exclusively until Solve returns.
			bp := &Problem{
				A: mat.NewDense(m, n, buf.aData),
				B: mat.NewVecDense(m, buf.bData),
				// Quality intentionally nil: bootstrap only needs X,
				// and skipping AnalyzeQuality avoids a full SVD per sample.
			}

			bSol, err := solver.Solve(bp)
			if err != nil {
				return nil // skip failed bootstrap samples
			}
			row := make([]float64, n)
			for j := 0; j < n; j++ {
				row[j] = bSol.X.AtVec(j)
			}
			samples[b] = row
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("bootstrap: %w", err)
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
