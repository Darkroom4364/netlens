package tomo

import (
	"math"
	"math/rand"
	"testing"
	"gonum.org/v1/gonum/mat"
)

func fullSV(A *mat.Dense) []float64 {
	var s mat.SVD
	s.Factorize(A, mat.SVDNone)
	v := make([]float64, min(A.RawMatrix().Rows, A.RawMatrix().Cols))
	return s.Values(v)
}

func TestRSVDKnownMatrix(t *testing.T) {
	A := mat.NewDense(6, 4, []float64{1, 0, 0, 0, 0, 2, 0, 0, 0, 0, 3, 0, 0, 0, 0, 4, 1, 1, 1, 1, 2, 2, 2, 2})
	want := fullSV(A)
	_, sv, _ := RandomizedSVD(A, 4, 10, rand.New(rand.NewSource(42)))
	for i := range sv {
		if math.Abs(sv[i]-want[i]) > 0.1 {
			t.Errorf("sv[%d]: got %.4f, want %.4f", i, sv[i], want[i])
		}
	}
}

func TestRSVDLarge(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	data := make([]float64, 500*200)
	for i := range data { data[i] = rng.NormFloat64() }
	_, sv, _ := RandomizedSVD(mat.NewDense(500, 200, data), 10, 10, rng)
	if len(sv) != 10 { t.Fatalf("expected 10 sv, got %d", len(sv)) }
	for i := 1; i < len(sv); i++ {
		if sv[i] > sv[i-1]+1e-9 { t.Errorf("sv not non-increasing at %d", i) }
	}
}

func TestRSVDClampK(t *testing.T) {
	_, sv, _ := RandomizedSVD(mat.NewDense(3, 2, []float64{1, 2, 3, 4, 5, 6}), 100, 10, rand.New(rand.NewSource(1)))
	if len(sv) != 2 { t.Errorf("expected clamped to 2, got %d", len(sv)) }
}

func TestRSVDK1(t *testing.T) {
	A := mat.NewDense(4, 3, []float64{1, 0, 0, 0, 2, 0, 0, 0, 3, 1, 1, 1})
	want := fullSV(A)
	_, sv, _ := RandomizedSVD(A, 1, 10, rand.New(rand.NewSource(99)))
	if math.Abs(sv[0]-want[0]) > 0.2 { t.Errorf("largest sv: got %.4f, want %.4f", sv[0], want[0]) }
}
