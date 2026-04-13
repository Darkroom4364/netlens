package cli

import (
	"fmt"
	"math"
	"path/filepath"

	"github.com/Darkroom4364/netlens/internal/measure"
	"github.com/Darkroom4364/netlens/internal/tomo"
	"github.com/Darkroom4364/netlens/internal/topology"
	"github.com/spf13/cobra"
)

func newSimulateCmd() *cobra.Command {
	var (
		topoFile        string
		method          string
		noiseScale      float64
		noiseModel      string
		congestionLinks int
		congestionFactor float64
		samplesPerPath  int
		seed            int64
		pathFraction    float64
	)

	cmd := &cobra.Command{
		Use:   "simulate",
		Short: "Run tomography on a simulated network with known ground truth",
		Long: `Loads a Topology Zoo GraphML file, assigns synthetic link delays with
configurable noise, runs the selected solver, and reports accuracy against
the known ground truth.`,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			solver := getSolver(method)
			sol, err := solver.Solve(sim.Problem)
			if err != nil {
				return fmt.Errorf("solve: %w", err)
			}

			// Warn about negative delay estimates (skip for NNLS which guarantees non-negativity)
			if method != "nnls" {
				negCount := 0
				for i := 0; i < sol.X.Len(); i++ {
					if sol.X.AtVec(i) < 0 {
						negCount++
					}
				}
				if negCount > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %d links have negative delay estimates (physically impossible). Consider using --method nnls to enforce non-negativity.\n", negCount)
				}
			}

			// Print results
			topoName := filepath.Base(topoFile)
			q := sim.Problem.Quality

			fmt.Printf("Topology:       %s (%d nodes, %d links)\n", topoName, g.NumNodes(), g.NumLinks())
			fmt.Printf("Paths:          %d\n", sim.Problem.NumPaths())
			fmt.Printf("Matrix rank:    %d / %d (identifiable: %.0f%%)\n",
				q.Rank, q.NumLinks, q.IdentifiableFrac*100)
			fmt.Printf("Condition:      %.2f\n", q.ConditionNumber)
			fmt.Printf("Solver:         %s\n", sol.Method)
			fmt.Printf("Duration:       %v\n", sol.Duration)
			fmt.Printf("Residual:       %.6f\n\n", sol.Residual)

			// Per-link comparison
			fmt.Printf("%-6s %-10s %-10s %-10s %-8s\n", "Link", "Truth(ms)", "Est(ms)", "Error(ms)", "Ident")
			fmt.Println("------------------------------------------------------")
			var sumSqErr, sumAbsErr float64
			identCount := 0
			for i, gt := range sim.GroundTruth {
				est := sol.X.AtVec(i)
				diff := est - gt
				ident := "yes"
				if !q.IsIdentifiable(i) {
					ident = "NO"
				} else {
					identCount++
					sumSqErr += diff * diff
					sumAbsErr += math.Abs(diff)
				}
				fmt.Printf("%-6d %-10.3f %-10.3f %-+10.3f %-8s\n", i, gt, est, diff, ident)
			}

			if identCount > 0 {
				rmse := math.Sqrt(sumSqErr / float64(identCount))
				mae := sumAbsErr / float64(identCount)
				fmt.Printf("\nRMSE (identifiable): %.4f ms\n", rmse)
				fmt.Printf("MAE  (identifiable): %.4f ms\n", mae)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&topoFile, "topology", "t", "", "Path to Topology Zoo GraphML file (required)")
	cmd.Flags().StringVarP(&method, "method", "m", "tikhonov", "Solver method: tsvd, tikhonov, nnls, admm, vardi, tomogravity")
	cmd.Flags().Float64Var(&noiseScale, "noise", 0.1, "Noise scale (relative, e.g., 0.1 = 10%)")
	cmd.Flags().StringVar(&noiseModel, "noise-model", "lognormal", "Noise model: lognormal, gaussian")
	cmd.Flags().IntVar(&congestionLinks, "congestion-links", 2, "Number of congested links")
	cmd.Flags().Float64Var(&congestionFactor, "congestion-factor", 5.0, "Delay multiplier for congested links")
	cmd.Flags().IntVar(&samplesPerPath, "samples", 3, "RTT samples per path (uses minimum)")
	cmd.Flags().Int64Var(&seed, "seed", 42, "Random seed (0 = random)")
	cmd.Flags().Float64Var(&pathFraction, "path-fraction", 1.0, "Fraction of all-pairs paths to use")
	_ = cmd.MarkFlagRequired("topology")

	return cmd
}

func getSolver(name string) tomo.Solver {
	switch name {
	case "tsvd":
		return &tomo.TSVDSolver{}
	case "nnls":
		return &tomo.NNLSSolver{}
	case "admm":
		return &tomo.ADMMSolver{}
	case "vardi":
		return &tomo.VardiEMSolver{}
	case "tomogravity":
		return &tomo.TomogravitySolver{}
	default:
		return &tomo.TikhonovSolver{}
	}
}
