package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func SetVersion(v string) { version = v }

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "netlens",
		Short: "Network tomography toolkit — infer what you can't observe",
		Long: `netlens infers internal network state (per-link latency, loss, congestion)
from edge measurements alone — without access to internal routers.

Feed it traceroutes. See the invisible.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	root.AddCommand(newVersionCmd())
	root.AddCommand(newSimulateCmd())
	root.AddCommand(newBenchmarkCmd())

	return root
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print netlens version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("netlens %s\n", version)
		},
	}
}
