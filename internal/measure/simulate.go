package measure

import (
	"math"
	"math/rand"

	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/internal/topology"
)

// SimConfig controls synthetic measurement generation.
type SimConfig struct {
	// NoiseScale is the relative noise (e.g., 0.1 = 10% of true delay).
	NoiseScale float64
	// NoiseModel: "lognormal" (default) or "gaussian".
	NoiseModel string
	// CongestionLinks is the number of links with elevated delay.
	CongestionLinks int
	// CongestionFactor is the multiplier for congested links.
	CongestionFactor float64
	// SamplesPerPath is how many RTT samples per path (uses minimum).
	SamplesPerPath int
	// Seed for reproducibility. 0 = random.
	Seed int64
	// PathFraction: fraction of all-pairs paths to use (0-1, default 1.0).
	PathFraction float64
}

// DefaultSimConfig returns sensible defaults.
func DefaultSimConfig() SimConfig {
	return SimConfig{
		NoiseScale:       0.1,
		NoiseModel:       "lognormal",
		CongestionLinks:  2,
		CongestionFactor: 5.0,
		SamplesPerPath:   3,
		Seed:             42,
		PathFraction:     1.0,
	}
}

// SimResult contains ground truth and the constructed problem for validation.
type SimResult struct {
	Problem     *tomo.Problem
	GroundTruth []float64 // True per-link delays in ms
	Paths       []tomo.PathSpec
	NoiseFree   []float64 // Noise-free end-to-end measurements
}

// Simulate generates synthetic measurements from a topology.
func Simulate(topo *topology.Graph, cfg SimConfig) (*SimResult, error) {
	rng := rand.New(rand.NewSource(cfg.Seed))
	if cfg.Seed == 0 {
		rng = rand.New(rand.NewSource(rand.Int63()))
	}

	nLinks := topo.NumLinks()
	links := topo.Links()
	nodes := topo.Nodes()

	// Generate ground truth delays based on geographic distance
	groundTruth := make([]float64, nLinks)
	for i, link := range links {
		srcNode := nodes[link.Src]
		dstNode := nodes[link.Dst]
		dist := topology.GeoDistance(srcNode, dstNode)
		if dist > 0 {
			// ~5 μs/km propagation delay → convert to ms
			groundTruth[i] = dist * 0.005
		} else {
			// No coordinates — assign random delay 1-10ms
			groundTruth[i] = 1.0 + rng.Float64()*9.0
		}
	}

	// Inject congestion
	if cfg.CongestionLinks > 0 && cfg.CongestionFactor > 1.0 {
		congestedIdx := rng.Perm(nLinks)
		for i := 0; i < min(cfg.CongestionLinks, nLinks); i++ {
			groundTruth[congestedIdx[i]] *= cfg.CongestionFactor
		}
	}

	// Generate paths
	allPaths := topo.AllPairsShortestPaths()
	paths := allPaths
	if cfg.PathFraction > 0 && cfg.PathFraction < 1.0 {
		n := int(float64(len(allPaths)) * cfg.PathFraction)
		if n < 1 {
			n = 1
		}
		perm := rng.Perm(len(allPaths))
		paths = make([]tomo.PathSpec, n)
		for i := 0; i < n; i++ {
			paths[i] = allPaths[perm[i]]
		}
	}

	// Compute noise-free measurements
	noiseFree := make([]float64, len(paths))
	for i, p := range paths {
		for _, linkID := range p.LinkIDs {
			noiseFree[i] += groundTruth[linkID]
		}
	}

	// Generate noisy measurements (minimum of multiple samples)
	measurements := make([]float64, len(paths))
	for i, nf := range noiseFree {
		bestSample := math.Inf(1)
		nSamples := cfg.SamplesPerPath
		if nSamples < 1 {
			nSamples = 1
		}
		for s := 0; s < nSamples; s++ {
			sample := addNoise(nf, cfg.NoiseScale, cfg.NoiseModel, rng)
			if sample < bestSample {
				bestSample = sample
			}
		}
		measurements[i] = bestSample
	}

	p, err := tomo.BuildProblem(topo, paths, measurements)
	if err != nil {
		return nil, err
	}

	return &SimResult{
		Problem:     p,
		GroundTruth: groundTruth,
		Paths:       paths,
		NoiseFree:   noiseFree,
	}, nil
}

// addNoise adds noise to a measurement value.
func addNoise(value, scale float64, model string, rng *rand.Rand) float64 {
	if scale <= 0 {
		return value
	}

	switch model {
	case "gaussian":
		noise := rng.NormFloat64() * scale * value
		result := value + noise
		if result < 0 {
			result = 0
		}
		return result

	default: // "lognormal"
		// Log-normal: the measurement is value * exp(N(0, σ))
		// where σ = scale. This is always positive.
		sigma := scale
		logNoise := rng.NormFloat64() * sigma
		return value * math.Exp(logNoise)
	}
}
