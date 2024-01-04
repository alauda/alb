package ingressclass

import (
	"testing"

	"alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"

	. "alauda.io/alb2/pkg/config"
	cfg "alauda.io/alb2/pkg/operator/config"
)

func TestIngressClassCtl(t *testing.T) {
	base := InitBase()
	l := log.InitKlogV2(log.LogCfg{ToFile: base + "/port-test.log"})
	l.Info("test ingress class ctl")
	ctl := IngClsCtl{}
	config := cfg.Config{
		Operator: cfg.DEFAULT_OPERATOR_CFG,
		ALB: cfg.ALB2Config{
			ALBRunConfig: ALBRunConfig{Ns: "default", Name: "alb", Controller: ControllerConfig{
				Flags: ControllerFlags{EnableIngress: true},
			}},
			Project:     cfg.ProjectConfig{},
			Flags:       cfg.OperatorFlags{DefaultIngressClass: false},
			ExtraConfig: cfg.ExtraConfig{},
		},
	}
	type Tcase struct {
		project       cfg.ProjectConfig
		httpport      int
		httspport     int
		expectProject string
	}
	cases := []Tcase{
		{
			project: cfg.ProjectConfig{
				Projects: []string{"p1", "p2"},
			},
			expectProject: "p1,p2",
		},
		// portProjects: '[{"port":"1-2","projects":["ALL_ALL"]},{"port":"3-4","projects":["cong","e2eproject"]}]'
		{
			project: cfg.ProjectConfig{
				EnablePortProject: true,
				PortProjects:      MarshOrPanic(PortProject{{Port: "80", Projects: []string{"p1"}}}),
			},
			expectProject: "",
		},
		{
			project: cfg.ProjectConfig{
				EnablePortProject: true,
				PortProjects: MarshOrPanic(PortProject{
					{Port: "80", Projects: []string{"ALL_ALL"}},
					{Port: "443", Projects: []string{"ALL_ALL"}},
				}),
			},
			expectProject: "ALL_ALL",
		},
		{
			project: cfg.ProjectConfig{
				EnablePortProject: true,
				PortProjects: MarshOrPanic(PortProject{
					{Port: "80", Projects: []string{"p1"}},
					{Port: "443", Projects: []string{"ALL_ALL"}},
				}),
			},
			expectProject: "p1",
		},
		{
			project: cfg.ProjectConfig{
				EnablePortProject: true,
				PortProjects:      MarshOrPanic(PortProject{{Port: "80-443", Projects: []string{"p1"}}}),
			},
			expectProject: "p1",
		},
	}
	for i, c := range cases {
		config.ALB.Project = c.project
		if c.httpport == 0 {
			c.httpport = 80
		}
		if c.httspport == 0 {
			c.httspport = 443
		}
		config.ALB.Controller.HttpPort = c.httpport
		config.ALB.Controller.HttpsPort = c.httspport
		ingcls, err := ctl.GenExpectIngressClass(nil, &config)
		assert.NoError(t, err)
		actual := ingcls.Annotations["alb.cpaas.io/project"]
		l.Info("ingcls", "i", i, "actual", actual, "expect", c.expectProject, "ingcls", PrettyCr(ingcls))
		assert.Equal(t, c.expectProject, actual)
	}
}
