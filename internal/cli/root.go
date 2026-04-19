package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Darkroom4364/netlens/internal/style"
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

	root.PersistentFlags().Bool("no-color", false, "Disable colored output")
	root.PersistentFlags().Bool("quiet", false, "Suppress verbose output, show only summary and results")

	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		loadDotEnv()
		noColor, _ := cmd.Flags().GetBool("no-color")
		if noColor {
			style.SetEnabled(false)
		}
	}

	root.AddCommand(newVersionCmd())
	root.AddCommand(newSimulateCmd())
	root.AddCommand(newBenchmarkCmd())
	root.AddCommand(newScanCmd())
	root.AddCommand(newPlanCmd())
	if cmd := newTUICmd(); cmd != nil {
		root.AddCommand(cmd)
	}
	root.AddCommand(newCompletionCmd())

	return root
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "completion [bash|zsh|fish]",
		Short:     "Generate shell completion script",
		ValidArgs: []string{"bash", "zsh", "fish"},
		Args:      cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
}

// loadDotEnv reads a .env file and sets any variables not already in the environment.
func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 {
			if (v[0] == '"' && v[len(v)-1] == '"') ||
				(v[0] == '\'' && v[len(v)-1] == '\'') {
				v = v[1 : len(v)-1]
			}
		}
		// Don't override existing env vars.
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
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
