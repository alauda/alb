package cmd

import (
	"context"
	"fmt"
	_ "fmt"
	"os"
	"sort"

	"alauda.io/alb2/config"
	"github.com/jedib0t/go-pretty/table"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/labels"

	av1 "alauda.io/alb2/pkg/apis/alauda/v1"
	corev1 "k8s.io/api/core/v1"
	ctrcli "sigs.k8s.io/controller-runtime/pkg/client"
)

var ruleCmd = &cobra.Command{
	Use:   "rule",
	Short: "info about alb rule",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		alb, err := cmd.Flags().GetString("alb")
		if err != nil {
			return err
		}
		Ropt.AlbName, Ropt.AlbNs = parseAlbKey(alb)
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		rules, err := listRule(cmd.Context(), Ropt)
		if err != nil {
			return err
		}
		output(rules)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(ruleCmd)
	flags := ruleCmd.Flags()
	flags.String("alb", "", "which alb you want inspect")
}

type RuleOpt struct {
	AlbName string
	AlbNs   string
}

var Ropt = RuleOpt{}

type Rule struct {
	raw    *av1.Rule
	Ns     string
	Name   string
	Match  string
	Svcs   []Svc
	Source Source
}

type Svc struct {
	Name string
	Ns   string
	Ep   []string
}
type Source struct {
	Name string
	Inex string
}

func listRule(ctx context.Context, opt RuleOpt) ([]Rule, error) {
	cli, err := getClient(ctx)
	if err != nil {
		return nil, err
	}
	rls := &av1.RuleList{}
	n := config.NewNames("cpaas.io")
	sel := labels.Set{
		n.GetLabelAlbName(): opt.AlbName,
	}.AsSelector()
	ccli := cli.GetClient()
	err = ccli.List(ctx, rls, &ctrcli.ListOptions{LabelSelector: sel, Namespace: opt.AlbNs})
	if err != nil {
		return nil, err
	}
	frls := []Rule{}
	for _, rule := range rls.Items {
		svcs := []Svc{}

		for _, svc := range rule.Spec.ServiceGroup.Services {
			if rule.Spec.RedirectCode != 0 {
				break
			}
			ep := corev1.Endpoints{}
			err := ccli.Get(ctx, ctrcli.ObjectKey{Namespace: svc.Namespace, Name: svc.Name}, &ep)
			if err != nil {
				fmt.Fprintf(os.Stderr, "get ep fail %v %v", svc, err)
				continue
			}
			svcs = append(svcs, Svc{Name: svc.Name, Ns: svc.Namespace, Ep: epToIpList(&ep)})
		}
		ur := rule
		r := Rule{
			raw:   &ur,
			Ns:    rule.Namespace,
			Name:  rule.Name,
			Match: rule.Spec.DSLX.ToSearchableString(),
			Svcs:  svcs,
		}
		if r.raw.Spec.Source.Type == "ingress" {
			pindex := r.raw.Annotations[n.GetLabelSourceIngressPathIndex()]
			rindex := r.raw.Annotations[n.GetLabelSourceIngressRuleIndex()]
			r.Source = Source{
				Name: r.raw.Spec.Source.Name,
				Inex: fmt.Sprintf("%s/%s", pindex, rindex),
			}
		}
		frls = append(frls, r)
	}
	sort.Slice(frls, func(left, right int) bool {
		lr := frls[left]
		rr := frls[right]
		lrp := lr.raw.Spec.Priority
		rrp := rr.raw.Spec.Priority
		ldslp := lr.raw.Spec.DSLX.Priority()
		rdslp := rr.raw.Spec.DSLX.Priority()
		llen := len(lr.raw.Spec.DSLX.ToSearchableString())
		rlen := len(rr.raw.Spec.DSLX.ToSearchableString())
		if lrp != rrp {
			return lrp < rrp
		}
		if ldslp != rdslp {
			return ldslp < rdslp
		}
		if llen != rlen {
			return llen > rlen
		}
		return lr.Name < rr.Name
	})
	return frls, nil
}

func epToIpList(ep *corev1.Endpoints) []string {
	ipList := []string{}
	for _, sub := range ep.Subsets {
		for _, addr := range sub.Addresses {
			ipList = append(ipList, addr.IP)
		}
	}
	return ipList
}

func output(rls []Rule) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Namespace", "Name", "Match", "Svcs", "Source"})
	for _, r := range rls {
		t.AppendRow(table.Row{r.Ns, r.Name, r.Match, r.Svcs, r.Source})
	}
	t.Render()
}
