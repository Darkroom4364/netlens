package tomo

import (
	"math"
	"math/rand"
	"testing"

	"gonum.org/v1/gonum/mat"
)

func TestIdentityFullRank(t *testing.T) {
	// Square identity matrix (full rank) → 0 unidentifiable links.
	n := 10
	data := make([]float64, n*n)
	for i := 0; i < n; i++ {
		data[i*n+i] = 1.0
	}
	A := mat.NewDense(n, n, data)
	q := AnalyzeQuality(A)

	if len(q.UnidentifiableLinks) != 0 {
		t.Errorf("expected 0 unidentifiable links for identity matrix, got %d", len(q.UnidentifiableLinks))
	}
	if q.Rank != n {
		t.Errorf("expected rank %d, got %d", n, q.Rank)
	}
}

func TestIdentityRankDeficient(t *testing.T) {
	// Structured rank-deficient matrix where specific links are unidentifiable.
	// Links 0-3 each have a dedicated measurement row (identifiable).
	// Links 4-5 only appear together, so they share a null space direction.
	A := mat.NewDense(5, 6, []float64{
		1, 0, 0, 0, 0, 0, // measures link 0
		0, 1, 0, 0, 0, 0, // measures link 1
		0, 0, 1, 0, 0, 0, // measures link 2
		0, 0, 0, 1, 0, 0, // measures link 3
		0, 0, 0, 0, 1, 1, // measures link 4+5 together
	})

	q := AnalyzeQuality(A)

	if q.Rank != 5 {
		t.Errorf("expected rank 5, got %d", q.Rank)
	}
	// Links 4 and 5 are unidentifiable (only their sum is observed).
	if len(q.UnidentifiableLinks) != 2 {
		t.Errorf("expected 2 unidentifiable links, got %d: %v", len(q.UnidentifiableLinks), q.UnidentifiableLinks)
	}
}

func TestIdentityLargeRankDeficient(t *testing.T) {
	// 100×500 random matrix with rank 100 → all 500 links unidentifiable
	// (random null space vectors generically involve all links).
	m, n := 100, 500
	rng := rand.New(rand.NewSource(99))
	data := make([]float64, m*n)
	for i := range data {
		data[i] = rng.NormFloat64()
	}
	A := mat.NewDense(m, n, data)

	q := AnalyzeQuality(A)

	if q.Rank != m {
		t.Errorf("expected rank %d, got %d", m, q.Rank)
	}
	// For a random matrix, all links participate in the null space.
	if len(q.UnidentifiableLinks) != n {
		t.Errorf("expected %d unidentifiable links, got %d", n, len(q.UnidentifiableLinks))
	}
}

func TestIdentityFracMatchesRank(t *testing.T) {
	// Full-rank square random matrix → IdentifiableFrac = 1.0.
	n := 30
	rng := rand.New(rand.NewSource(7))
	data := make([]float64, n*n)
	for i := range data {
		data[i] = rng.NormFloat64()
	}
	A := mat.NewDense(n, n, data)

	q := AnalyzeQuality(A)

	if q.Rank != n {
		t.Errorf("expected rank %d, got %d", n, q.Rank)
	}
	if math.Abs(q.IdentifiableFrac-1.0) > 1e-9 {
		t.Errorf("IdentifiableFrac = %f, expected 1.0 for full-rank matrix", q.IdentifiableFrac)
	}
}
