package plan

import (
	"github.com/Darkroom4364/netlens/internal/tomo"
	"gonum.org/v1/gonum/mat"
)

const svdTolerance = 1e-10

// ProbePair is a recommended source-destination probe with its rank gain.
type ProbePair struct {
	Src      int
	Dst      int
	RankGain int // how much this probe improves rank
}

// RecommendProbes suggests probe pairs that maximize rank(A).
// Given an existing Problem (or nil for fresh start), recommend
// up to budget new source-destination pairs.
func RecommendProbes(topo tomo.Topology, existing *tomo.Problem, budget int) []ProbePair {
	nLinks := topo.NumLinks()
	if nLinks == 0 || budget <= 0 {
		return nil
	}

	// Get all candidate paths.
	allPaths := topo.AllPairsShortestPaths()
	if len(allPaths) == 0 {
		return nil
	}

	// Initialize A from existing problem or empty.
	var rows [][]float64
	selected := make(map[int]bool) // indices into allPaths already in A

	if existing != nil {
		m, _ := existing.A.Dims()
		// Copy existing rows. Also mark any matching paths as selected.
		for i := 0; i < m; i++ {
			row := make([]float64, nLinks)
			for j := 0; j < nLinks; j++ {
				row[j] = existing.A.At(i, j)
			}
			rows = append(rows, row)
		}
		// Mark existing paths as selected by matching their routing vectors.
		for ci, cp := range allPaths {
			candidateRow := pathToRow(cp, nLinks)
			for i := 0; i < m; i++ {
				if rowsEqual(rows[i], candidateRow) {
					selected[ci] = true
					break
				}
			}
		}
	}

	currentRank := computeRank(rows, nLinks)

	var result []ProbePair

	for iter := 0; iter < budget; iter++ {
		bestGain := 0
		bestIdx := -1

		for ci, cp := range allPaths {
			if selected[ci] {
				continue
			}

			candidateRow := pathToRow(cp, nLinks)
			// Tentatively add.
			rows = append(rows, candidateRow)
			newRank := computeRank(rows, nLinks)
			gain := newRank - currentRank
			// Remove tentative row.
			rows = rows[:len(rows)-1]

			if gain > bestGain {
				bestGain = gain
				bestIdx = ci
			}
		}

		if bestIdx < 0 || bestGain == 0 {
			break // no more information to gain
		}

		// Add best path permanently.
		cp := allPaths[bestIdx]
		rows = append(rows, pathToRow(cp, nLinks))
		selected[bestIdx] = true
		currentRank += bestGain

		result = append(result, ProbePair{
			Src:      cp.Src,
			Dst:      cp.Dst,
			RankGain: bestGain,
		})
	}

	return result
}

// pathToRow converts a PathSpec into a binary routing row.
func pathToRow(p tomo.PathSpec, nLinks int) []float64 {
	row := make([]float64, nLinks)
	for _, lid := range p.LinkIDs {
		if lid >= 0 && lid < nLinks {
			row[lid] = 1.0
		}
	}
	return row
}

// rowsEqual checks if two float64 slices are identical.
func rowsEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// computeRank builds a matrix from rows and returns its numerical rank.
func computeRank(rows [][]float64, nCols int) int {
	m := len(rows)
	if m == 0 || nCols == 0 {
		return 0
	}

	data := make([]float64, m*nCols)
	for i, row := range rows {
		copy(data[i*nCols:], row)
	}

	A := mat.NewDense(m, nCols, data)

	var svd mat.SVD
	ok := svd.Factorize(A, mat.SVDNone)
	if !ok {
		return 0
	}

	sv := make([]float64, min(m, nCols))
	svd.Values(sv)

	if len(sv) == 0 || sv[0] == 0 {
		return 0
	}

	threshold := sv[0] * svdTolerance * float64(max(m, nCols))
	rank := 0
	for _, s := range sv {
		if s > threshold {
			rank++
		}
	}
	return rank
}

