package tomo

import (
	"math"

	"gonum.org/v1/gonum/mat"
)

const svdTolerance = 1e-10

// AnalyzeQuality computes the identifiability and conditioning of routing matrix A.
func AnalyzeQuality(A *mat.Dense) *MatrixQuality {
	m, n := A.Dims()

	// Compute SVD of A
	var svd mat.SVD
	ok := svd.Factorize(A, mat.SVDThin)
	if !ok {
		// SVD failed — return worst-case quality
		return &MatrixQuality{
			Rank:                0,
			NumLinks:            n,
			NumPaths:            m,
			ConditionNumber:     math.Inf(1),
			IdentifiableFrac:    0,
			UnidentifiableLinks: makeRange(0, n),
			CoveragePerLink:     computeCoverage(A),
		}
	}

	// Extract singular values
	sv := make([]float64, min(m, n))
	svd.Values(sv)

	// Compute rank: count singular values above tolerance
	maxSV := sv[0]
	threshold := maxSV * svdTolerance * float64(max(m, n))
	rank := 0
	for _, s := range sv {
		if s > threshold {
			rank++
		}
	}

	// Condition number: ratio of largest to smallest nonzero singular value
	condNumber := math.Inf(1)
	if rank > 0 {
		minNonzero := sv[0]
		for _, s := range sv[:rank] {
			if s < minNonzero {
				minNonzero = s
			}
		}
		if minNonzero > 0 {
			condNumber = maxSV / minNonzero
		}
	}

	// Identify unidentifiable links via the right singular vectors (V).
	// A link is unidentifiable if it has zero projection onto the column space of A,
	// i.e., the corresponding row of V (for the nonzero singular value columns) is all zeros.
	var v mat.Dense
	svd.VTo(&v)
	vRows, _ := v.Dims()

	unidentifiable := identifyNullSpaceLinks(v, rank, vRows)
	identifiableFrac := 1.0
	if n > 0 {
		identifiableFrac = float64(n-len(unidentifiable)) / float64(n)
	}

	return &MatrixQuality{
		Rank:                rank,
		NumLinks:            n,
		NumPaths:            m,
		ConditionNumber:     condNumber,
		IdentifiableFrac:    identifiableFrac,
		UnidentifiableLinks: unidentifiable,
		CoveragePerLink:     computeCoverage(A),
		SingularValues:      sv,
	}
}

// identifyNullSpaceLinks finds links whose rows in V are zero
// across all columns corresponding to nonzero singular values.
func identifyNullSpaceLinks(V mat.Dense, rank, nLinks int) []int {
	if rank == 0 {
		return makeRange(0, nLinks)
	}
	if rank >= nLinks {
		return nil // all links identifiable
	}

	var unidentifiable []int
	for i := 0; i < nLinks; i++ {
		norm := 0.0
		for j := 0; j < rank; j++ {
			v := V.At(i, j)
			norm += v * v
		}
		if math.Sqrt(norm) < svdTolerance {
			unidentifiable = append(unidentifiable, i)
		}
	}
	return unidentifiable
}

// computeCoverage counts how many paths traverse each link.
func computeCoverage(A *mat.Dense) []int {
	m, n := A.Dims()
	coverage := make([]int, n)
	for j := 0; j < n; j++ {
		for i := 0; i < m; i++ {
			if A.At(i, j) > 0 {
				coverage[j]++
			}
		}
	}
	return coverage
}

func makeRange(start, end int) []int {
	r := make([]int, end-start)
	for i := range r {
		r[i] = start + i
	}
	return r
}
