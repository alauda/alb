package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	av2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
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
	// case 1 rule has no ep
	cli, err := getClient(ctx)
	if err != nil {
		return err
	}
	albs := av2.ALB2List{}
	err = cli.GetClient().List(ctx, &albs)
	if err != nil {
		return err
	}
	for _, alb := range albs.Items {
		opt := RuleOpt{
			AlbName: alb.Name,
			AlbNs:   alb.Namespace,
		}
		rules, err := listRule(ctx, opt)
		if err != nil {
			return err
		}
		for _, rule := range rules {
			eps := []string{}
			for _, svc := range rule.Svcs {
				eps = append(eps, svc.Ep...)
			}
			if len(eps) == 0 && rule.raw.Spec.RedirectCode == 0 {
				fmt.Printf("has-no-ep alb %s/%s rule %s  match %s source %s svcs %s\n", opt.AlbNs, opt.AlbName, rule.Name, rule.Match, rule.Source, rule.Svcs)
			}
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(debug)
}
