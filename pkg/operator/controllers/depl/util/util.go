package util

import (
	"context"
	"fmt"
	"strings"

	. "alauda.io/alb2/pkg/operator/toolkit"
	u "alauda.io/alb2/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	perr "github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MigrateKind string

const (
	CreateIfNotExistKind       = MigrateKind("CreateIfNotExist")
	DeleteOrUpdateOrCreateKind = MigrateKind("DeleteOrUpdateOrCreate")
	CreateOrDeleteKind         = MigrateKind("CreateOrDelete")
)

func DeleteOrCreate[T client.Object](ctx context.Context, cli client.Client, log logr.Logger, cur T, expect T) error {
	return migrateCr(CreateOrDeleteKind, ctx, cli, log, cur, expect, func(cur T, expect T) bool { return true })
}

func DeleteOrCreateOrUpdate[T client.Object](ctx context.Context, cli client.Client, log logr.Logger, cur T, expect T, need func(cur, expect T) bool) error {
	return migrateCr(DeleteOrUpdateOrCreateKind, ctx, cli, log, cur, expect, need)
}

func migrateCr[T client.Object](kind MigrateKind, ctx context.Context, cli client.Client, log logr.Logger, cur T, expect T, need func(cur, expect T) bool) error {
	if !IsNil(cur) && IsNil(expect) {
		log.Info("do delete", "cur", ShowMeta(cur))
		err := cli.Delete(ctx, cur)
		if err != nil {
			return perr.Wrapf(err, "do delete %v fail %v", cur.GetObjectKind(), cur.GetName())
		}
		log.Info("do delete success", "cur", ShowMeta(expect))
	}
	if IsNil(cur) && !IsNil(expect) {
		log.Info("do create", "cr", PrettyCr(expect))
		err := cli.Create(ctx, expect)
		if err != nil {
			return perr.Wrapf(err, "create %v %v fail", expect.GetObjectKind(), expect.GetName())
		}
		log.Info("do create success", "expect", ShowMeta(expect))
	}
	if kind == CreateIfNotExistKind || kind == CreateOrDeleteKind {
		log.Info("do nothing.", "kind", kind, "cur", ShowMeta(cur), "expect", ShowMeta(expect))
		return nil
	}
	if !IsNil(cur) && !IsNil(expect) {
		if !need(cur, expect) {
			log.Info("same, ignore update.", "cur", ShowMeta(cur), "expect", ShowMeta(expect))
			return nil
		}
		log.Info("not same, do update.", "cur", ShowMeta(cur), "expect", ShowMeta(expect), "diff", cmp.Diff(cur, expect))
		err := cli.Update(ctx, expect)
		if err != nil {
			return perr.Wrapf(err, "update %v %v fail", expect.GetObjectKind(), expect.GetName())
		}
		log.Info("update success", "new", ShowMeta(expect))

		return nil
	}
	return nil
}

// 标识这个资源的alb
var (
	ALB2OperatorResourceLabelName   = "alb.cpaas.io/alb2-operator-albname"
	ALB2OperatorResourceLabelNs     = "alb.cpaas.io/alb2-operator-albns"
	ALB2OperatorResourceLabelLegacy = "alb.cpaas.io/alb2-operator"
	ALB2OperatorLabel               = "alb.cpaas.io/managed-by"
)

// 表示这个资源部署时operator的版本
var ALB2OperatorVersionLabel = "alb.cpaas.io/version"

// 原本是将ns和name放在同一个value中的,但是可以加起来就超过了63个字符了.所以分拆成两个label,为了保持兼容性,在小于63的时候,仍然加上legacy的label
// 目前只有lbsvc是用label来获取的

func LegacyALBLabel(ns, name string) map[string]string {
	val := fmt.Sprintf("%s_%s", ns, name)
	m := map[string]string{}
	m[ALB2OperatorResourceLabelLegacy] = val
	return m
}

func ALBLabel(ns, name string) map[string]string {
	val := fmt.Sprintf("%s_%s", ns, name)
	m := map[string]string{}
	if len(val) < 63 {
		m[ALB2OperatorResourceLabelLegacy] = val
	}
	m[ALB2OperatorResourceLabelName] = name
	m[ALB2OperatorResourceLabelNs] = ns
	return m
}

func GetAlbKeyFromObject(obj client.Object) (ns string, name string, version string, err error) {
	{
		find, ns, name, version, err := getAlbKeyFromObjectNew(obj)
		if err != nil {
			return "", "", "", err
		}
		if find {
			return ns, name, version, nil
		}
	}
	find, ns, name, version, err := getAlbKeyFromObjectLegacy(obj)
	if err != nil {
		return "", "", "", err
	}
	if find {
		return ns, name, version, nil
	}
	return "", "", "", fmt.Errorf("not managed this object")
}

func getAlbKeyFromObjectLegacy(obj client.Object) (find bool, ns string, name string, version string, err error) {
	labels := obj.GetLabels()
	key, keyOk := labels[ALB2OperatorResourceLabelLegacy]
	version, versionOk := labels[ALB2OperatorVersionLabel]
	if !keyOk || !versionOk {
		return false, "", "", "", fmt.Errorf("not alb-operator managed deployment")
	}
	ns, name, err = SplitMetaNamespaceKeyWith(key, "_")
	if err != nil {
		return false, "", "", "", fmt.Errorf("invalid key %s", key)
	}
	return true, ns, name, version, nil
}

func getAlbKeyFromObjectNew(obj client.Object) (find bool, ns string, name string, version string, err error) {
	labels := obj.GetLabels()
	name, namekeyOk := labels[ALB2OperatorResourceLabelName]
	ns, nskeyOk := labels[ALB2OperatorResourceLabelNs]
	version, versionOk := labels[ALB2OperatorVersionLabel]
	if !namekeyOk || nskeyOk || !versionOk {
		return false, "", "", "", nil
	}
	return true, ns, name, version, nil
}

func SplitMetaNamespaceKeyWith(key, sep string) (namespace, name string, err error) {
	parts := strings.Split(key, sep)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid key format: %s", key)
	}
	return parts[0], parts[1], nil
}

func ALB2ResourceLabel(ns, name string, version string) map[string]string {
	return MergeMap(ALBLabel(ns, name), OperatorLabel(version))
}

func OperatorLabel(version string) map[string]string {
	return map[string]string{
		ALB2OperatorVersionLabel: version,
		ALB2OperatorLabel:        "alb-operator",
	}
}

func MergeMap(a map[string]string, b map[string]string) map[string]string {
	return u.MergeMap(a, b)
}

// 如果key中有某个prefix，就删除这个key
func RemovePrefixKey(m map[string]string, prefix string) map[string]string {
	ret := map[string]string{}
	for k, v := range m {
		if strings.HasPrefix(k, prefix) {
			continue
		}
		ret[k] = v
	}
	return ret
}
