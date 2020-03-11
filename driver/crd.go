package driver

import (
	"alauda.io/alb2/config"
	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"time"

	"github.com/golang/glog"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	CrdVersion = "v1"
	CrdScope   = apiextensionsv1beta1.NamespaceScoped
	// 	GroupName = "crd.alauda.io"
	CrdGroupName = v1.GroupName
)

type Crd struct {
	Kind       string
	ListKind   string
	Plural     string
	Singular   string
	ShortNames []string
}

var CrdTypes = []Crd{
	{
		Kind:       "ALB2",
		ListKind:   "ALB2List",
		Plural:     "alaudaloadbalancer2",
		Singular:   "alaudaloadbalancer2",
		ShortNames: []string{"alb2"},
	},
	{
		Kind:       "Frontend",
		ListKind:   "FrontendList",
		Plural:     "frontends",
		Singular:   "frontend",
		ShortNames: []string{"ft"},
	},
	{
		Kind:       "Rule",
		ListKind:   "RuleList",
		Plural:     "rules",
		Singular:   "rule",
		ShortNames: []string{"rl"},
	},
}

func (d *KubernetesDriver) RegisterCustomDefinedResources() error {
	skipCreate := true
	for _, crdType := range CrdTypes {
		name := crdType.Plural + "." + CrdGroupName
		crd, errGet := d.ExtClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(name, meta_v1.GetOptions{})
		if errGet != nil {
			skipCreate = false
			break // create the resources
		}
		for _, cond := range crd.Status.Conditions {
			if cond.Type == apiextensionsv1beta1.Established &&
				cond.Status == apiextensionsv1beta1.ConditionTrue {
				continue
			}

			if cond.Type == apiextensionsv1beta1.NamesAccepted &&
				cond.Status == apiextensionsv1beta1.ConditionTrue {
				continue
			}

			glog.Warningf("Not established: %v", name)
			skipCreate = false
			break
		}
	}

	if skipCreate {
		return nil
	}

	for _, crdType := range CrdTypes {
		name := crdType.Plural + "." + CrdGroupName
		crd := &apiextensionsv1beta1.CustomResourceDefinition{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: name,
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group:   CrdGroupName,
				Version: CrdVersion,
				Scope:   CrdScope,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Singular:   crdType.Singular,
					Plural:     crdType.Plural,
					Kind:       crdType.Kind,
					ListKind:   crdType.ListKind,
					ShortNames: crdType.ShortNames,
				},
				// TODO: add validation
				Validation: nil,
			},
		}
		glog.Infof("registering CRD %q", name)
		_, err := d.ExtClient.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}

	// wait for CRD being established
	errPoll := wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
	LOOP:
		for _, crdType := range CrdTypes {
			name := crdType.Plural + "." + CrdGroupName
			crd, errGet := d.ExtClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(name, meta_v1.GetOptions{})
			if errGet != nil {
				return false, errGet
			}
			if config.Get("TEST") != "true" {
				for _, cond := range crd.Status.Conditions {
					switch cond.Type {
					case apiextensionsv1beta1.Established:
						if cond.Status == apiextensionsv1beta1.ConditionTrue {
							glog.Infof("established CRD %q", name)
							continue LOOP
						}
					case apiextensionsv1beta1.NamesAccepted:
						if cond.Status == apiextensionsv1beta1.ConditionFalse {
							glog.Warningf("name conflict: %v", cond.Reason)
						}
					}
				}
			} else {
				continue LOOP
			}
			glog.Infof("missing status condition for %q", name)
			return false, nil
		}
		return true, nil
	})

	if errPoll != nil {
		return errPoll
	}

	return nil
}
