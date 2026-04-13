package tomo

import (
	"fmt"

	"gonum.org/v1/gonum/mat"
)

// SVDResult holds the factorized components of a matrix.
type SVDResult struct {
	U  *mat.Dense
	Sv []float64
	V  *mat.Dense
}

// FactorizeSVD computes the full SVD of A and returns U, singular values, and V.
func FactorizeSVD(A *mat.Dense) (*SVDResult, error) {
	var svd mat.SVD
	if !svd.Factorize(A, mat.SVDFull) {
		return nil, fmt.Errorf("svd: factorization failed")
	}
	m, n := A.Dims()
	sv := make([]float64, min(m, n))
	svd.Values(sv)
	var u, v mat.Dense
	svd.UTo(&u)
	svd.VTo(&v)
	return &SVDResult{U: &u, Sv: sv, V: &v}, nil
}

// BuildIdentifiabilityMask returns a boolean slice indicating which links
// are identifiable based on the MatrixQuality analysis.
func BuildIdentifiabilityMask(q *MatrixQuality, n int) []bool {
	mask := make([]bool, n)
	if q != nil {
		for i := range mask {
			mask[i] = q.IsIdentifiable(i)
		}
	}
	return mask
}

// ComputeResidual returns ||Ax - b||₂.
func ComputeResidual(A *mat.Dense, x *mat.VecDense, b *mat.VecDense) float64 {
	return computeResidual(A, x, b)
}
