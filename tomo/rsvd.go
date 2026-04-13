package tomo

import (
	"math/rand"
	"gonum.org/v1/gonum/mat"
)

// RandomizedSVD computes an approximate rank-k SVD of A (Halko et al. 2011).
// Returns U (m×k), singular values (k), V (n×k).
func RandomizedSVD(A *mat.Dense, k, oversample int, rng *rand.Rand) (*mat.Dense, []float64, *mat.Dense) {
	m, n := A.Dims()
	if oversample <= 0 {
		oversample = 10
	}
	l := min(k+oversample, min(m, n))
	k = min(k, l)
	// Random Gaussian Ω (n×l) and Y = AΩ
	omega := mat.NewDense(n, l, nil)
	for i := range n {
		for j := range l {
			omega.Set(i, j, rng.NormFloat64())
		}
	}
	var Y mat.Dense
	Y.Mul(A, omega)
	// QR of Y → thin Q (m×l)
	var qr mat.QR
	qr.Factorize(&Y)
	var qFull mat.Dense
	qr.QTo(&qFull)
	Q := mat.NewDense(m, l, nil)
	for i := range m {
		for j := range l {
			Q.Set(i, j, qFull.At(i, j))
		}
	}
	// B = QᵀA, SVD(B), then U = QÛ truncated to k
	var B mat.Dense
	B.Mul(Q.T(), A)
	var svd mat.SVD
	svd.Factorize(&B, mat.SVDFull)
	vals := make([]float64, min(l, n))
	svd.Values(vals)
	var uHat, vMat mat.Dense
	svd.UTo(&uHat)
	svd.VTo(&vMat)
	var uFull mat.Dense
	uFull.Mul(Q, &uHat)
	U, V := mat.NewDense(m, k, nil), mat.NewDense(n, k, nil)
	for i := range m {
		for j := range k {
			U.Set(i, j, uFull.At(i, j))
		}
	}
	for i := range n {
		for j := range k {
			V.Set(i, j, vMat.At(i, j))
		}
	}
	return U, vals[:k], V
}
