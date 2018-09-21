package driver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	typev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"alauda_lb/config"
)

func TestCreateDriver(t *testing.T) {
	a := assert.New(t)

	server := "http://127.0.0.1:6443"
	token := "test-token"
	config.Set("KUBERNETES_SERVER", server)
	config.Set("KUBERNETES_BEARERTOKEN", token)
	drv, err := GetDriver()
	a.NoError(err)
	kdrv, ok := drv.(*KubernetesDriver)
	a.True(ok)
	a.Equal(server, kdrv.Endpoint)
	a.Equal(token, kdrv.BearerToken)
	a.NotZero(kdrv.Timeout)
}

func setUp() {
	config.Set("k8s_v3", "true")
	config.InitLabels()
}

func getFakeDriver(t *testing.T) *KubernetesDriver {
	a := assert.New(t)
	config.Set("KUBERNETES_SERVER", FAKE_ENDPOINT)
	drv, err := GetDriver()
	a.NoError(err)
	kdrv, ok := drv.(*KubernetesDriver)
	a.True(ok)
	a.NotNil(kdrv)
	return kdrv
}

func TestGetEndpoint(t *testing.T) {
	setUp()
	a := assert.New(t)
	kdrv := getFakeDriver(t)
	client := kdrv.Client
	ep := typev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Endpoints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s1",
			Namespace: "default",
			Labels: map[string]string{
				"service.alauda.io/uuid": "f0dafa83-4f2e-4edc-9c3d-e2a4c9ebf297",
			},
		},
		Subsets: []typev1.EndpointSubset{},
	}
	_, err := client.CoreV1().Endpoints("default").Create(&ep)
	a.NoError(err)

	services, err := kdrv.ListServiceEndpoints()
	a.NoError(err)
	a.NotEmpty(services)
}
