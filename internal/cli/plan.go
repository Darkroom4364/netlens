package cli

import (
	"fmt"
	"path/filepath"

	"github.com/Darkroom4364/netlens/internal/plan"
	"github.com/Darkroom4364/netlens/internal/style"
	"github.com/Darkroom4364/netlens/topology"
	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	var (
		topoFile string
		budget   int
	)

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Recommend probe pairs to maximize routing matrix rank",
		Long: `Greedy measurement design: given a network topology, recommend which
source-destination pairs to probe in order to maximize the rank of the
routing matrix A. Higher rank means more links are identifiable.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := topology.LoadGraphML(topoFile)
			if err != nil {
				return fmt.Errorf("load topology: %w", err)
			}

			topoName := filepath.Base(topoFile)
			fmt.Printf("%s  %s (%d nodes, %d links)\n", style.Bold("Topology:"), topoName, g.NumNodes(), g.NumLinks())
			fmt.Printf("%s    %d probes\n\n", style.Bold("Budget:"), budget)

			probes := plan.RecommendProbes(g, nil, budget)

			if len(probes) == 0 {
				fmt.Println("No useful probes found.")
				return nil
			}

			// Print table.
			fmt.Printf("%-5s  %-5s  %-5s  %-10s  %-10s\n", "Step", "Src", "Dst", "RankGain", "CumRank")
			fmt.Println("-------------------------------------------")

			cumRank := 0
			for i, p := range probes {
				cumRank += p.RankGain
				gainStr := fmt.Sprintf("%d", p.RankGain)
				if p.RankGain > 0 {
					gainStr = style.Green(gainStr)
				} else {
					gainStr = style.Dim(gainStr)
				}
				fmt.Printf("%-5d  %-5d  %-5d  %s  %-10d\n", i+1, p.Src, p.Dst, style.PadRight(gainStr, 10), cumRank)
			}

			fmt.Println("\n" + style.Bold(fmt.Sprintf("Total probes: %d, final rank: %d / %d links", len(probes), cumRank, g.NumLinks())))
			if cumRank == g.NumLinks() {
				fmt.Println("Full rank achieved: all links identifiable.")
			} else {
				fmt.Printf("Rank deficit: %d links remain unidentifiable.\n", g.NumLinks()-cumRank)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&topoFile, "topology", "t", "", "Path to Topology Zoo GraphML file (required)")
	cmd.Flags().IntVarP(&budget, "budget", "b", 20, "Maximum probe pairs to recommend")
	_ = cmd.MarkFlagRequired("topology")

	return cmd
}
