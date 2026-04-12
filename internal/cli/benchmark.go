package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Darkroom4364/netlens/internal/bench"
	"github.com/Darkroom4364/netlens/internal/measure"
	"github.com/Darkroom4364/netlens/internal/tomo"
	"github.com/Darkroom4364/netlens/internal/topology"
	"github.com/spf13/cobra"
)

func newBenchmarkCmd() *cobra.Command {
	var (
		topoDir          string
		noiseScale       float64
		noiseModel       string
		congestionLinks  int
		congestionFactor float64
		seed             int64
	)

	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Run all solvers on all topologies and compare accuracy",
		Long: `Loads all GraphML files from a directory, simulates measurements with
configurable noise, runs every solver, and produces a comparison table.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := filepath.Glob(filepath.Join(topoDir, "*.graphml"))
			if err != nil {
				return fmt.Errorf("glob topologies: %w", err)
			}
			if len(files) == 0 {
				return fmt.Errorf("no .graphml files found in %s", topoDir)
			}

			cfg := measure.SimConfig{
				NoiseScale:       noiseScale,
				NoiseModel:       noiseModel,
				CongestionLinks:  congestionLinks,
				CongestionFactor: congestionFactor,
				SamplesPerPath:   3,
				Seed:             seed,
				PathFraction:     1.0,
			}

			solvers := []tomo.Solver{
				&tomo.TSVDSolver{},
				&tomo.TikhonovSolver{},
				&tomo.NNLSSolver{},
				&tomo.ADMMSolver{},
				&tomo.VardiEMSolver{},
			}

			var allResults []bench.BenchResult

			for _, f := range files {
				g, err := topology.LoadGraphML(f)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: skip %s: %v\n", f, err)
					continue
				}

				name := strings.TrimSuffix(filepath.Base(f), ".graphml")
				results, err := bench.RunBenchmark(name, g, solvers, cfg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: %s: %v\n", name, err)
					continue
				}
				allResults = append(allResults, results...)
			}

			fmt.Print(bench.FormatResults(allResults))
			return nil
		},
	}

	cmd.Flags().StringVarP(&topoDir, "topologies", "t", "testdata/topologies", "Directory containing .graphml files")
	cmd.Flags().Float64Var(&noiseScale, "noise", 0.1, "Noise scale (relative)")
	cmd.Flags().StringVar(&noiseModel, "noise-model", "lognormal", "Noise model: lognormal, gaussian")
	cmd.Flags().IntVar(&congestionLinks, "congestion-links", 2, "Number of congested links")
	cmd.Flags().Float64Var(&congestionFactor, "congestion-factor", 5.0, "Congestion delay multiplier")
	cmd.Flags().Int64Var(&seed, "seed", 42, "Random seed")

	return cmd
}
