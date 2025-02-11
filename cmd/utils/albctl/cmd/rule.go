package cmd

import (
	"context"
	"fmt"
	_ "fmt"
	"os"
	"path"
	"strings"

	. "alauda.io/alb2/controller/cli"
	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	ing "alauda.io/alb2/ingress"
	. "alauda.io/alb2/utils"
	"alauda.io/alb2/utils/log"
	"github.com/go-logr/logr"
	"github.com/markkurossi/tabulate"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var RuleCmd = &cobra.Command{
	Use:   "rule",
	Short: "info about alb rule",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := NewCtx(FLAG, cmd.Context())
		if err != nil {
			return err
		}
		err = listRule(*ctx)
		if err != nil {
			return err
		}
		return nil
	},
}

var CheckIngressCmd = &cobra.Command{
	Use:   "check-ingress",
	Short: "check alb should handle this ingress",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := NewCtx(FLAG, cmd.Context())
		if err != nil {
			return err
		}
		err = checkingress(*ctx)
		if err != nil {
			return err
		}
		return nil
	},
}

var ListRuleCmd = &cobra.Command{
	Use:   "list",
	Short: "list all rules of a alb",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := NewCtx(FLAG, cmd.Context())
		if err != nil {
			return err
		}
		err = listRule(*ctx)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(RuleCmd)
	RuleCmd.AddCommand(CheckIngressCmd)
	RuleCmd.AddCommand(ListRuleCmd)
	flags := RuleCmd.PersistentFlags()
	flags.StringVar(&FLAG.AlbName, "name", "", "name of alb you want inspect")
	flags.StringVar(&FLAG.AlbNs, "ns", "cpaas-system", "ns of alb you want inspect")
	flags.StringVar(&FLAG.Kubecfgpath, "kubeconfig", "", "specified kubeconfig or $KUBECONFIG or ~/.kube/config")
	flags.IntVar(&FLAG.loglevel, "lv", 1, "log level, 0 to disable log")
	ListRuleCmd.Flags().BoolVar(&FLAG.JsonMode, "json", false, "output full json")
	CheckIngressCmd.Flags().BoolVar(&FLAG.all, "all", false, "check this ingress in all alb")
	CheckIngressCmd.Flags().StringVar(&FLAG.ingKey, "ingress", "", "ns/name of ingress")
}

type Ctx struct {
	ctx         context.Context
	AlbName     string
	AlbNs       string
	Kubecfgpath string
	Kubecfg     *rest.Config
	JsonMode    bool
	loglevel    int
	log         logr.Logger
	all         bool
	ingStr      string
}
type Flags struct {
	AlbName     string
	AlbNs       string
	Kubecfgpath string
	JsonMode    bool
	loglevel    int
	all         bool
	ingKey      string
}

var FLAG = Flags{}

func kubecfgfromfile(p string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: p},
		nil,
	).ClientConfig()
}

func (c *Ctx) tryKubecfg(p string) (*rest.Config, error) {
	cf, err := kubecfgfromfile(p)
	if err == nil {
		return cf, nil
	}
	cf, err = kubecfgfromfile(os.Getenv("KUBECONFIG"))
	if err == nil {
		return cf, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	defaultPath := path.Join(home, ".kube/config")
	cf, err = kubecfgfromfile(defaultPath)
	if err == nil {
		return cf, nil
	}
	return nil, fmt.Errorf("no kubeconfig found")
}

func NewCtx(flag Flags, ctx context.Context) (*Ctx, error) {
	log.InitKlogV2(log.LogCfg{Level: fmt.Sprintf("%d", flag.loglevel)})
	nctx := Ctx{
		ctx:         ctx,
		AlbName:     flag.AlbName,
		AlbNs:       flag.AlbNs,
		log:         log.L(),
		Kubecfgpath: flag.Kubecfgpath,
		loglevel:    flag.loglevel,
		JsonMode:    flag.JsonMode,
		ingStr:      flag.ingKey,
		all:         flag.all,
	}
	err := nctx.init()
	if err != nil {
		return nil, err
	}
	return &nctx, nil
}

func (ctx *Ctx) init() error {
	cf, err := ctx.tryKubecfg(ctx.Kubecfgpath)
	if err != nil {
		return err
	}
	ctx.Kubecfg = cf
	return nil
}

func parse_host(dsl albv1.DSLX) string {
	host := ""
	for _, term := range dsl {
		if term.Type == albv1.KEY_HOST {
			host += term.Values[0][1]
		}
	}
	return host
}

func listRule(ctx Ctx) error {
	drv, err := driver.NewDriver(driver.DrvOpt{
		Ctx: ctx.ctx,
		Cf:  ctx.Kubecfg,
		Opt: driver.Opt{
			Domain: "cpaas.io",
			Ns:     ctx.AlbNs,
		},
	})
	if err != nil {
		return err
	}
	acli := NewAlbCli(drv, ctx.log)
	pcli := NewPolicyCli(drv, ctx.log, PolicyCliOpt{MetricsPort: 0})
	if ctx.AlbName == "" {
		albs, err := drv.ALBClient.CrdV2beta1().ALB2s(ctx.AlbNs).List(ctx.ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		names := strings.Join(lo.Map(albs.Items, func(a albv2.ALB2, _ int) string { return a.Name }), ",")
		fmt.Printf("alb names: %s use --name to select one of them\n", names)
		return nil
	}

	lb, err := acli.GetLBConfig(ctx.AlbNs, ctx.AlbName)
	if err != nil {
		return err
	}
	rule_maps := map[string]InternalRule{}
	for _, ft := range lb.Frontends {
		for _, r := range ft.Rules {
			rule_maps[r.RuleID] = *r
		}
	}
	err = pcli.FillUpBackends(lb)
	if err != nil {
		return err
	}
	policy := pcli.GenerateAlbPolicy(lb)
	backens_maps := map[string]BackendGroup{}
	for _, b := range policy.BackendGroup {
		backens_maps[b.Name] = *b
	}

	type Record struct {
		Ns           string
		Name         string
		Port         int
		RealPriority int
		Host         string
		Matches      string
		Policy       Policy
		Upstream     BackendGroup
	}
	rs := map[string]Record{}
	for port, policies := range policy.Http.Tcp {
		for _, p := range policies {
			rs[p.Rule] = Record{
				Ns:           ctx.AlbNs,
				Name:         p.Rule,
				Port:         int(port),
				RealPriority: p.ComplexPriority,
				Host:         parse_host(rule_maps[p.Rule].DSLX),
				Matches:      PrettyCompactJson(p.InternalDSL),
				Policy:       *p,
				Upstream:     backens_maps[p.Rule],
			}
		}
	}
	if ctx.JsonMode {
		fmt.Print(PrettyJson(rs))
		return nil
	}
	tab := tabulate.New(tabulate.Plain)
	for _, h := range []string{"NAMESPACE", "PORT", "NAME", "R_PRIORITY", "HOST", "MATCHES", "SOURCE", "UPSTREAM"} {
		tab.Header(h)
	}
	for _, r := range rs {
		row := tab.Row()
		rows := []string{
			r.Ns,
			fmt.Sprintf("%d", r.Port),
			r.Name,
			fmt.Sprintf("%d", r.RealPriority),
			r.Host,
			PrettyCompactJson(r.Policy.InternalDSL),
			PrettyCompactJson(r.Policy.Source),
			strings.Join(lo.Map(r.Upstream.Backends, func(b *Backend, _ int) string {
				return fmt.Sprintf("%s %s:%d %d", b.Pod, b.Address, b.Port, b.Weight)
			}), ", "),
		}
		for _, c := range rows {
			row.Column(c)
		}
	}
	tab.Print(os.Stdout)
	return nil
}

func checkingress(ctx Ctx) error {
	drv, err := driver.NewDriver(driver.DrvOpt{
		Ctx: ctx.ctx,
		Cf:  ctx.Kubecfg,
		Opt: driver.Opt{
			Domain: "cpaas.io",
			Ns:     ctx.AlbNs,
		},
	})
	if err != nil {
		return err
	}
	ingkey, err := ParseStringToObjectKey(ctx.ingStr)
	if err != nil {
		return err
	}
	if ctx.AlbName == "" && !ctx.all {
		albs, err := drv.ALBClient.CrdV2beta1().ALB2s(ctx.AlbNs).List(ctx.ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		names := strings.Join(lo.Map(albs.Items, func(a albv2.ALB2, _ int) string { return a.Name }), ",")
		fmt.Printf("alb names: %s use --name to select one of them. or use --all to check this ingress in all alb\n", names)
		return nil
	}
	checkingress := func(albkey client.ObjectKey, ingkey client.ObjectKey) error {
		alb, err := drv.LoadALB(types.NamespacedName{Namespace: albkey.Namespace, Name: albkey.Name})
		if err != nil {
			return err
		}
		ingcr, err := drv.Client.NetworkingV1().Ingresses(ingkey.Namespace).Get(ctx.ctx, ingkey.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		should, reason := ing.NewIngressSelect(ing.IngressSelectOpt{
			HttpPort:  NullOr(alb.Alb.Spec.Config.IngressHTTPPort, 80),
			HttpsPort: NullOr(alb.Alb.Spec.Config.IngressHTTPPort, 443),
			Domain:    "cpaas.io",
			Name:      alb.Alb.Name,
		}, drv).ShouldHandleIngress(alb, ingcr)
		fmt.Println("alb:", albkey, "ing:", ingkey, "should-handle-ingress:", should, "reason:", reason)
		return nil
	}
	if ctx.AlbName == "" && ctx.all {
		albs, err := drv.ALBClient.CrdV2beta1().ALB2s("").List(ctx.ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, alb := range albs.Items {
			err := checkingress(client.ObjectKeyFromObject(&alb), ingkey)
			if err != nil {
				return err
			}
		}
		return nil
	}

	err = checkingress(client.ObjectKey{Name: ctx.AlbName, Namespace: ctx.AlbNs}, ingkey)
	if err != nil {
		return err
	}
	return nil
}
