package controller

import (
	"alb2/config"
	"alb2/driver"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetTestDriver() *driver.KubernetesDriver {
	kd, _ := driver.GetKubernetesDriver(driver.FAKE_ENDPOINT, "", 0)
	return kd
}

func TestUpdateServiceBind(t *testing.T) {
	a := assert.New(t)
	kd := GetTestDriver()
	a.NotNil(kd)
	bindList := []BindInfo{
		BindInfo{
			Name:          "alb2",
			Port:          80,
			Protocol:      ProtocolHTTP,
			ContainerPort: 8080,
		},
	}
	js, _ := json.Marshal(bindList)

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
			Annotations: map[string]string{
				config.Get("labels.bindkey"): string(js),
			},
		},
	}
	nsvc, err := kd.Client.CoreV1().Services(svc.Namespace).Create(svc)
	a.NoError(err)
	a.NotNil(nsvc)

	requests, err := ListBindRequest(kd)
	a.NoError(err)
	a.Empty(requests)

	config.Set("NAME", "alb2")
	requests, err = ListBindRequest(kd)
	a.NoError(err)
	a.NotEmpty(requests)
	req := requests[0]
	a.Equal("alb2", req.Name)
	a.Equal(svc.Name, req.ServiceName)
	a.Equal(svc.Namespace, req.Namespace)

	bind := *req
	bind.State = StateReady
	bind.ResourceName = "alb2-80-abcd"
	bind.ResourceType = TypeRule

	err = UpdateServiceBind(kd, &bind)
	a.NoError(err)

	nsvc, err = kd.Client.CoreV1().Services(svc.Namespace).Get(svc.Name, metav1.GetOptions{})
	a.NoError(err)
	a.NotNil(nsvc)

	var nbl []BindInfo
	err = json.Unmarshal([]byte(nsvc.Annotations[config.Get("labels.bindkey")]), &nbl)
	a.NoError(err)
	a.Equal(bind.State, nbl[0].State)
	a.Equal(bind.ResourceName, nbl[0].ResourceName)
	a.Equal(bind.ResourceType, nbl[0].ResourceType)
}
