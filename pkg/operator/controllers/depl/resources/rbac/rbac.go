package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	_ "embed"

	cfg "alauda.io/alb2/pkg/operator/config"
	. "alauda.io/alb2/pkg/operator/controllers/depl/resources/types"
	. "alauda.io/alb2/pkg/operator/controllers/depl/util"
	. "alauda.io/alb2/pkg/operator/toolkit"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed alb-clusterrole.json
var CLUSTER_RULE_YAML string

type RbacCtl struct {
	ctx context.Context
	cli crcli.Client
	log logr.Logger
	cfg cfg.Config
}

type CurRbac struct {
	ClusterRole     *rbacv1.ClusterRole
	ClusterRoleBind *rbacv1.ClusterRoleBinding
	ServiceAccount  *corev1.ServiceAccount
}

func (c *CurRbac) GetObjs() []crcli.Object {
	return []crcli.Object{
		c.ClusterRole,
		c.ClusterRoleBind,
		c.ServiceAccount,
	}
}

func NewRbacCtl(ctx context.Context, cli crcli.Client, log logr.Logger, cfg cfg.Config) *RbacCtl {
	return &RbacCtl{
		ctx: ctx,
		cli: cli,
		log: log,
		cfg: cfg,
	}
}

func Load(ctx context.Context, cli crcli.Client, log logr.Logger, albkey crcli.ObjectKey) (*CurRbac, error) {
	role := &rbacv1.ClusterRole{}
	rolebind := &rbacv1.ClusterRoleBinding{}
	sa := &corev1.ServiceAccount{}
	err := cli.Get(ctx, crcli.ObjectKey{Name: fmt.Sprintf(FMT_ROLE, albkey.Name)}, role)
	if err != nil && k8serrors.IsNotFound(err) {
		role = nil
		err = nil
	}
	if err != nil {
		return nil, err
	}

	err = cli.Get(ctx, crcli.ObjectKey{Name: fmt.Sprintf(FMT_ROLEBIND, albkey.Name)}, rolebind)
	if err != nil && k8serrors.IsNotFound(err) {
		rolebind = nil
		err = nil
	}
	if err != nil {
		return nil, err
	}
	err = cli.Get(ctx, crcli.ObjectKey{Name: fmt.Sprintf(FMT_SA, albkey.Name), Namespace: albkey.Namespace}, sa)
	if err != nil && k8serrors.IsNotFound(err) {
		log.Info("sa not found", "err", err)
		sa = nil
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return &CurRbac{
		ClusterRole:     role,
		ClusterRoleBind: rolebind,
		ServiceAccount:  sa,
	}, nil
}

type RbacUpdate CurRbac

func (r *RbacCtl) GenUpdate(cur *CurRbac) *RbacUpdate {
	return &RbacUpdate{
		ClusterRole:     r.GenClusterRole(cur.ClusterRole.DeepCopy()),
		ClusterRoleBind: r.GenClusterRoleBind(cur.ClusterRoleBind.DeepCopy()),
		ServiceAccount:  r.GenServiceAccount(cur.ServiceAccount.DeepCopy()),
	}
}

func rulesOrPanic() []rbacv1.PolicyRule {
	role := &rbacv1.ClusterRole{}
	err := json.Unmarshal([]byte(CLUSTER_RULE_YAML), role)
	if err != nil {
		panic(err)
	}
	return role.Rules
}

func (r *RbacCtl) GenClusterRole(role *rbacv1.ClusterRole) *rbacv1.ClusterRole {
	if IsNil(role) {
		role = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf(FMT_ROLE, r.cfg.ALB.Name),
			},
		}
	}
	refLabel := ALB2ResourceLabel(r.cfg.ALB.Ns, r.cfg.ALB.Name, r.cfg.Operator.Version)
	role.Rules = rulesOrPanic()
	role.Labels = MergeMap(role.Labels, refLabel)
	return role
}

func (r *RbacCtl) GenClusterRoleBind(role *rbacv1.ClusterRoleBinding) *rbacv1.ClusterRoleBinding {
	rolebind := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf(FMT_ROLEBIND, r.cfg.ALB.Name)},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      fmt.Sprintf(FMT_SA, r.cfg.ALB.Name),
				Namespace: r.cfg.ALB.Ns,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     fmt.Sprintf(FMT_ROLE, r.cfg.ALB.Name),
		},
	}
	refLabel := ALB2ResourceLabel(r.cfg.ALB.Ns, r.cfg.ALB.Name, r.cfg.Operator.Version)
	rolebind.Labels = MergeMap(rolebind.Labels, refLabel)
	return rolebind
}

func (r *RbacCtl) GenServiceAccount(sa *corev1.ServiceAccount) *corev1.ServiceAccount {
	if sa == nil {
		sa = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf(FMT_SA, r.cfg.ALB.Name),
				Namespace: r.cfg.ALB.Ns,
			},
		}
	}
	refLabel := ALB2ResourceLabel(r.cfg.ALB.Ns, r.cfg.ALB.Name, r.cfg.Operator.Version)
	sa.Labels = MergeMap(sa.Labels, refLabel)
	return sa
}

func (r *RbacCtl) DoUpdate(cur *CurRbac, expect *RbacUpdate) error {
	if cur == nil || expect == nil {
		return fmt.Errorf("cur or expect is nil %v %v", cur, expect)
	}
	err := DeleteOrCreateOrUpdate(r.ctx, r.cli, r.log, cur.ClusterRole, expect.ClusterRole, func(cur, expect *rbacv1.ClusterRole) bool {
		r.log.Info("cmp rule", "diff", cmp.Diff(cur.Rules, expect.Rules), "cur-ver", cur.ResourceVersion)
		rulesEq := reflect.DeepEqual(cur.Rules, expect.Rules)
		labelEq := reflect.DeepEqual(cur.Labels, expect.Labels)
		same := rulesEq && labelEq
		return !same
	})
	if err != nil {
		return err
	}
	err = DeleteOrCreateOrUpdate(r.ctx, r.cli, r.log, cur.ClusterRoleBind, expect.ClusterRoleBind, func(cur, expect *rbacv1.ClusterRoleBinding) bool {
		subjectEq := reflect.DeepEqual(cur.Subjects, expect.Subjects)
		roleEq := reflect.DeepEqual(cur.RoleRef, expect.RoleRef)
		labelEq := reflect.DeepEqual(cur.Labels, expect.Labels)
		same := subjectEq && labelEq && roleEq
		return !same
	})
	if err != nil {
		return err
	}
	err = DeleteOrCreateOrUpdate(r.ctx, r.cli, r.log, cur.ServiceAccount, expect.ServiceAccount, func(cur, expect *corev1.ServiceAccount) bool {
		return !reflect.DeepEqual(cur.Labels, expect.Labels)
	})
	if err != nil {
		return err
	}
	return nil
}
