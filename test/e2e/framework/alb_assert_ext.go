package framework

import (
	"context"
	"fmt"

	a1t "alauda.io/alb2/pkg/apis/alauda/v1"
	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	. "alauda.io/alb2/utils/test_utils"
)

type AlbHelper struct {
	*K8sClient
	AlbInfo
}

func (f *AlbHelper) CreateFt(port a1t.PortNumber, protocol string, svcName string, svcNs string) {
	name := fmt.Sprintf("%s-%05d", f.AlbName, port)
	if protocol == "udp" {
		name += "-udp"
	}
	ft := a1t.Frontend{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.AlbNs,
			Name:      name,
			Labels: map[string]string{
				"alb2.cpaas.io/name": f.AlbName,
			},
		},
		Spec: a1t.FrontendSpec{
			Port:     port,
			Protocol: a1t.FtProtocol(protocol),
			ServiceGroup: &a1t.ServiceGroup{Services: []a1t.Service{
				{
					Name:      svcName,
					Namespace: svcNs,
					Port:      80,
				},
			}},
		},
	}
	f.GetAlbClient().CrdV1().Frontends(f.AlbNs).Create(context.Background(), &ft, metav1.CreateOptions{})
}

func (f *AlbHelper) WaitFtState(name string, check func(ft *a1t.Frontend) (bool, error)) *a1t.Frontend {
	var ft *a1t.Frontend
	var err error
	err = wait.Poll(Poll, DefaultTimeout, func() (bool, error) {
		ft, err = f.GetAlbClient().CrdV1().Frontends(f.AlbNs).Get(context.Background(), name, metav1.GetOptions{})
		Logf("try get ft %s/%s ft %v", f.AlbNs, name, err)
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		ok, err := check(ft)
		if err == nil {
			return ok, nil
		}
		return ok, err
	})
	assert.NoError(ginkgo.GinkgoT(), err)
	return ft
}

func (f *AlbHelper) WaitFt(name string) *a1t.Frontend {
	return f.WaitFtState(name, func(ft *a1t.Frontend) (bool, error) {
		if ft != nil {
			return true, nil
		}
		return false, nil
	})
}
