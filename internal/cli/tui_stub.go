//go:build !tui

package cli

import "github.com/spf13/cobra"

func newTUICmd() *cobra.Command { return nil }
