package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// case1. rule has no ep
// case2. rule start with /

var debug = &cobra.Command{
	Use:   "debug",
	Short: "auto debug",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		now := time.Now()
		defer func() {
			fmt.Printf("over spend %v\n", time.Since(now))
		}()
		return autodebug(ctx)
	},
}

func autodebug(ctx context.Context) error {
	return nil
}

func init() {
	rootCmd.AddCommand(debug)
}
