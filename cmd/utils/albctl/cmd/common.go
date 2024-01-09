package cmd

import (
	"context"
	"strings"

	. "alauda.io/alb2/utils/test_utils"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func parseAlbKey(alb string) (string, string) {
	if strings.Contains(alb, "/") {
		ns := strings.Split(alb, "/")[0]
		name := strings.Split(alb, "/")[1]
		return name, ns
	} else {
		return alb, "cpaas-system"
	}
}

func resolveRestCfg() (*rest.Config, error) {
	return clientcmd.BuildConfigFromFlags("", GOpt.KubecfgPath)
}

func getClient(ctx context.Context) (*K8sClient, error) {
	cfg, err := resolveRestCfg()
	if err != nil {
		return nil, err
	}
	cli := NewK8sClient(ctx, cfg)
	return cli, nil
}
