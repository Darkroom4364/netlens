package cli

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"

	"github.com/Darkroom4364/netlens/internal/measure"
	"github.com/Darkroom4364/netlens/internal/style"
	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/topology"
	"github.com/spf13/cobra"
)

func newSimulateCmd() *cobra.Command {
	var (
		topoFile         string
		method           string
		noiseScale       float64
		noiseModel       string
		congestionLinks  int
		congestionFactor float64
		samplesPerPath   int
		seed             int64
		pathFraction     float64
		top              int
	)

	cmd := &cobra.Command{
		Use:   "simulate",
		Short: "Run tomography on a simulated network with known ground truth",
		Long: `Loads a Topology Zoo GraphML file, assigns synthetic link delays with
configurable noise, runs the selected solver, and reports accuracy against
the known ground truth.`,
		Example: `  netlens simulate -t testdata/topologies/Abilene.graphml
  netlens simulate -t network.graphml -m nnls --noise 0.2 --top 10
  netlens simulate -t network.graphml -m admm --congestion-links 5 --seed 0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if noiseScale < 0 {
				return fmt.Errorf("--noise must be >= 0 (got %.4f)", noiseScale)
			}
			if pathFraction <= 0 || pathFraction > 1 {
				return fmt.Errorf("--path-fraction must be in (0, 1] (got %.4f)", pathFraction)
			}
			if congestionFactor <= 0 {
				return fmt.Errorf("--congestion-factor must be > 0 (got %.4f)", congestionFactor)
			}
			if samplesPerPath < 1 {
				return fmt.Errorf("--samples must be >= 1 (got %d)", samplesPerPath)
			}

			g, err := topology.LoadGraphML(topoFile)
			if err != nil {
				return fmt.Errorf("load topology: %w", err)
			}

			cfg := measure.SimConfig{
				NoiseScale:       noiseScale,
				NoiseModel:       noiseModel,
				CongestionLinks:  congestionLinks,
				CongestionFactor: congestionFactor,
				SamplesPerPath:   samplesPerPath,
				Seed:             seed,
				PathFraction:     pathFraction,
			}

			sim, err := measure.Simulate(g, cfg)
			if err != nil {
				return fmt.Errorf("simulate: %w", err)
			}

			solver, err := getSolver(method)
			if err != nil {
				return err
			}
			sol, err := solver.Solve(sim.Problem)
			if err != nil {
				return fmt.Errorf("solve: %w", err)
			}

			warnNegativeDelays(cmd, sol, method)

			// Print results
			topoName := filepath.Base(topoFile)
			q := sim.Problem.Quality
			p := sim.Problem

			quiet, _ := cmd.Flags().GetBool("quiet")
			if !quiet {
				fmt.Printf("Topology:       %s (%d nodes, %d links)\n", topoName, g.NumNodes(), g.NumLinks())
				fmt.Printf("Paths:          %d\n", sim.Problem.NumPaths())
				fmt.Printf("Matrix rank:    %d / %d (identifiable: %.0f%%)\n",
					q.Rank, q.NumLinks, q.IdentifiableFrac*100)
				fmt.Printf("Condition:      %.2f\n", q.ConditionNumber)
				fmt.Printf("Solver:         %s\n", sol.Method)
				fmt.Printf("Duration:       %v\n", sol.Duration)
				fmt.Printf("Residual:       %.6f\n", sol.Residual)
			}

			// Compute error metrics first for summary line
			var sumSqErr, sumAbsErr float64
			identCount := 0
			for i, gt := range sim.GroundTruth {
				est := sol.X.AtVec(i)
				diff := est - gt
				if q.IsIdentifiable(i) {
					identCount++
					sumSqErr += diff * diff
					sumAbsErr += math.Abs(diff)
				}
			}

			var rmse, mae float64
			if identCount > 0 {
				rmse = math.Sqrt(sumSqErr / float64(identCount))
				mae = sumAbsErr / float64(identCount)
			}

			// Summary line
			congested := 0
			for i := 0; i < p.NumLinks(); i++ {
				if sol.X.AtVec(i) > style.DelayCongestionMS {
					congested++
				}
			}
			fmt.Printf("\n%s  %s  %s  %s\n\n",
				style.Bold(fmt.Sprintf("%d links", p.NumLinks())),
				style.Yellow(fmt.Sprintf("%d congested", congested)),
				fmt.Sprintf("RMSE %.2fms", rmse),
				fmt.Sprintf("%.0f%% identifiable", q.IdentifiableFrac*100))

			// Sort links by delay descending
			indices := make([]int, len(sim.GroundTruth))
			for i := range indices {
				indices[i] = i
			}
			sort.Slice(indices, func(a, b int) bool {
				return sol.X.AtVec(indices[a]) > sol.X.AtVec(indices[b])
			})
			if top > 0 && top < len(indices) {
				indices = indices[:top]
			}

			// Per-link comparison
			fmt.Printf("%s\n", style.Bold(fmt.Sprintf("%-6s %-10s %-10s %-10s %-8s", "Link", "Truth(ms)", "Est(ms)", "Error(ms)", "Ident")))
			fmt.Println("------------------------------------------------------")
			for _, i := range indices {
				gt := sim.GroundTruth[i]
				est := sol.X.AtVec(i)
				diff := est - gt
				ident := style.ColorIdent(q.IsIdentifiable(i))
				fmt.Printf("%-6d %s %s %-+10.3f %s\n", i, style.PadRight(style.ColorDelay(gt), 10), style.PadRight(style.ColorDelay(est), 10), diff, style.PadRight(ident, 8))
			}

			if identCount > 0 {
				fmt.Printf("\nRMSE (identifiable): %.4f ms\n", rmse)
				fmt.Printf("MAE  (identifiable): %.4f ms\n", mae)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&topoFile, "topology", "t", "", "Path to Topology Zoo GraphML file (required)")
	cmd.Flags().StringVarP(&method, "method", "m", "tikhonov", "Solver method: tsvd, tikhonov, nnls, admm, irl1, vardi, tomogravity, laplacian")
	cmd.Flags().Float64Var(&noiseScale, "noise", 0.1, "Noise scale (relative, e.g., 0.1 = 10%)")
	cmd.Flags().StringVar(&noiseModel, "noise-model", "lognormal", "Noise model: lognormal, gaussian")
	cmd.Flags().IntVar(&congestionLinks, "congestion-links", 2, "Number of congested links")
	cmd.Flags().Float64Var(&congestionFactor, "congestion-factor", 5.0, "Delay multiplier for congested links")
	cmd.Flags().IntVar(&samplesPerPath, "samples", 3, "RTT samples per path (uses minimum)")
	cmd.Flags().Int64Var(&seed, "seed", 42, "Random seed (0 = random)")
	cmd.Flags().Float64Var(&pathFraction, "path-fraction", 1.0, "Fraction of all-pairs paths to use")
	cmd.Flags().IntVar(&top, "top", 0, "Show only the N worst links (0 = show all)")
	_ = cmd.MarkFlagRequired("topology")

	return cmd
}

// warnNegativeDelays prints a warning to stderr if the solution contains negative estimates.
func warnNegativeDelays(cmd *cobra.Command, sol *tomo.Solution, method string) {
	if method == "nnls" {
		return
	}
	negCount := 0
	for i := 0; i < sol.X.Len(); i++ {
		if sol.X.AtVec(i) < 0 {
			negCount++
		}
	}
	if negCount > 0 {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %d links have negative delay estimates (physically impossible). Consider using --method nnls to enforce non-negativity.\n", negCount)
	}
}

func getSolver(name string) (tomo.Solver, error) {
	switch name {
	case "tsvd":
		return &tomo.TSVDSolver{}, nil
	case "tikhonov":
		return &tomo.TikhonovSolver{}, nil
	case "nnls":
		return &tomo.NNLSSolver{}, nil
	case "admm":
		return &tomo.ADMMSolver{}, nil
	case "irl1":
		return &tomo.IRL1Solver{}, nil
	case "vardi":
		return &tomo.VardiEMSolver{}, nil
	case "tomogravity":
		return &tomo.TomogravitySolver{}, nil
	case "laplacian":
		return &tomo.LaplacianSolver{}, nil
	default:
		return nil, fmt.Errorf("unknown solver method %q; valid: tsvd, tikhonov, nnls, admm, irl1, vardi, tomogravity, laplacian", name)
	}
}
