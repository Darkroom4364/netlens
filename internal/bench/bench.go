package bench

import (
	"fmt"
	"math"
	"sort"

	"github.com/Darkroom4364/netlens/internal/measure"
	"github.com/Darkroom4364/netlens/internal/style"
	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/topology"
	"gonum.org/v1/gonum/mat"
)

// BenchResult holds the result of benchmarking one solver on one topology.
type BenchResult struct {
	Topology        string
	Solver          string
	NumNodes        int
	NumLinks        int
	NumPaths        int
	Rank            int
	ConditionNumber float64
	RMSE            float64 // root mean square error (identifiable links only)
	MAE             float64 // mean absolute error (identifiable links only)
	MaxRelErr       float64 // maximum relative error (identifiable links only)
	DetectionRate   float64 // fraction of top-k congested links correctly identified
	IdentifiablePct float64
	DurationMs      float64
}

// RunBenchmark runs all solvers on a loaded topology with simulation.
func RunBenchmark(topoName string, g *topology.Graph, solvers []tomo.Solver, cfg measure.SimConfig) ([]BenchResult, error) {
	sim, err := measure.Simulate(g, cfg)
	if err != nil {
		return nil, fmt.Errorf("simulate %s: %w", topoName, err)
	}

	var results []BenchResult
	for _, solver := range solvers {
		sol, err := solver.Solve(sim.Problem)
		if err != nil {
			return nil, fmt.Errorf("solver %s on %s: %w", solver.Name(), topoName, err)
		}

		result := evaluate(topoName, solver.Name(), sim, sol)
		results = append(results, result)
	}
	return results, nil
}

// evaluate computes error metrics for a solution against ground truth.
func evaluate(topoName, solverName string, sim *measure.SimResult, sol *tomo.Solution) BenchResult {
	gt := sim.GroundTruth
	nLinks := len(gt)
	quality := sim.Problem.Quality

	// Compute errors only on identifiable links
	var sumSqErr, sumAbsErr float64
	maxRelErr := 0.0
	identCount := 0

	for i := 0; i < nLinks; i++ {
		if !quality.IsIdentifiable(i) {
			continue
		}
		identCount++
		est := sol.X.AtVec(i)
		diff := est - gt[i]
		sumSqErr += diff * diff
		sumAbsErr += math.Abs(diff)
		if gt[i] > 0.001 { // skip near-zero ground truth to avoid division instability
			relErr := math.Abs(diff) / gt[i]
			if relErr > maxRelErr {
				maxRelErr = relErr
			}
		}
	}

	rmse := 0.0
	mae := 0.0
	if identCount > 0 {
		rmse = math.Sqrt(sumSqErr / float64(identCount))
		mae = sumAbsErr / float64(identCount)
	}

	// Detection rate: can we find the congested links?
	detectionRate := computeDetectionRate(gt, sol.X, quality, sim.Problem.Quality.CoveragePerLink)

	return BenchResult{
		Topology:        topoName,
		Solver:          solverName,
		NumNodes:        sim.Problem.Topo.NumNodes(),
		NumLinks:        nLinks,
		NumPaths:        sim.Problem.NumPaths(),
		Rank:            quality.Rank,
		ConditionNumber: quality.ConditionNumber,
		RMSE:            rmse,
		MAE:             mae,
		MaxRelErr:       maxRelErr,
		DetectionRate:   detectionRate,
		IdentifiablePct: quality.IdentifiableFrac * 100,
		DurationMs:      float64(sol.Duration.Microseconds()) / 1000.0,
	}
}

// computeDetectionRate checks if the solver identifies the top-k worst links.
// "Worst" = highest delay in ground truth. We check if the solver's top-k
// matches ground truth's top-k.
func computeDetectionRate(gt []float64, est *mat.VecDense, quality *tomo.MatrixQuality, coverage []int) float64 {
	n := len(gt)
	if n == 0 {
		return 0
	}

	// Only consider identifiable links
	type linkVal struct {
		idx int
		val float64
	}

	var gtLinks, estLinks []linkVal
	for i := 0; i < n; i++ {
		if !quality.IsIdentifiable(i) {
			continue
		}
		gtLinks = append(gtLinks, linkVal{i, gt[i]})
		estLinks = append(estLinks, linkVal{i, est.AtVec(i)})
	}

	if len(gtLinks) == 0 {
		return 0
	}

	sort.Slice(gtLinks, func(i, j int) bool { return gtLinks[i].val > gtLinks[j].val })
	sort.Slice(estLinks, func(i, j int) bool { return estLinks[i].val > estLinks[j].val })

	// Check top-k where k = min(3, len/4)
	k := min(3, len(gtLinks)/4)
	if k == 0 {
		k = 1
	}

	// Build ground truth top-k set
	topK := make(map[int]bool)
	for i := 0; i < k; i++ {
		topK[gtLinks[i].idx] = true
	}

	// Count how many of solver's top-k are in ground truth top-k
	hits := 0
	for i := 0; i < k && i < len(estLinks); i++ {
		if topK[estLinks[i].idx] {
			hits++
		}
	}

	return float64(hits) / float64(k)
}

// FormatResults produces a human-readable table of benchmark results.
func FormatResults(results []BenchResult) string {
	header := style.Bold(fmt.Sprintf("%-20s %-10s %5s %5s %5s %4s %8s %8s %8s %10s %6s %8s",
		"Topology", "Solver", "Nodes", "Links", "Paths", "Rank", "Cond", "RMSE", "MAE", "MaxRelErr", "Ident", "Time(ms)")) + "\n"
	divider := ""
	for i := 0; i < 110; i++ {
		divider += "-"
	}
	divider += "\n"

	out := header + divider
	for _, r := range results {
		rmseStr := fmt.Sprintf("%8.4f", r.RMSE)
		if math.IsNaN(r.RMSE) || math.IsInf(r.RMSE, 0) {
			rmseStr = fmt.Sprintf("%8s", "N/A")
		} else if r.RMSE < 5 {
			rmseStr = style.Green(rmseStr)
		} else if r.RMSE <= 20 {
			rmseStr = style.Yellow(rmseStr)
		} else {
			rmseStr = style.Red(rmseStr)
		}

		maeStr := fmt.Sprintf("%8.4f", r.MAE)
		if math.IsNaN(r.MAE) || math.IsInf(r.MAE, 0) {
			maeStr = fmt.Sprintf("%8s", "N/A")
		}

		maxRelStr := fmt.Sprintf("%9.2f%%", r.MaxRelErr*100)
		if math.IsNaN(r.MaxRelErr) || math.IsInf(r.MaxRelErr, 0) {
			maxRelStr = fmt.Sprintf("%10s", "N/A")
		}

		identStr := fmt.Sprintf("%5.0f%%", r.IdentifiablePct)
		if r.IdentifiablePct < 100 {
			identStr = style.Red(identStr)
		}
		out += fmt.Sprintf("%s %-10s %5d %5d %5d %4d %8.1f %s %s %s %s %8.2f\n",
			style.PadRight(style.Bold(r.Topology), 20), r.Solver, r.NumNodes, r.NumLinks, r.NumPaths,
			r.Rank, r.ConditionNumber, style.PadRight(rmseStr, 8), maeStr, maxRelStr,
			style.PadRight(identStr, 6), r.DurationMs)
	}
	return out
}
