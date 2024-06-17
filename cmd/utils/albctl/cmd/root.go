package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type GlobalOpt struct {
	KubecfgPath string
}

var GOpt = GlobalOpt{}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "albctl",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if GOpt.KubecfgPath != "" {
			return nil
		}
		env := os.Getenv("KUBECONFIG")
		if env != "" {
			GOpt.KubecfgPath = env
		}
		defaultPath := os.Getenv("HOME") + "/.kube/config"
		_, err := os.Stat(defaultPath)
		if GOpt.KubecfgPath == "" && err == nil {
			GOpt.KubecfgPath = defaultPath
		}
		if GOpt.KubecfgPath == "" {
			return fmt.Errorf("need kubecfg path")
		}
		return nil
	},
	Short: "debug util for alb",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
}
