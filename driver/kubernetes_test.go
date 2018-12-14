package driver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	typev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"alb2/config"
)

func TestCreateDriver(t *testing.T) {
	a := assert.New(t)

	config.Set("TEST", "true")
	drv, err := GetDriver()
	a.NoError(err)

	a.NotNil(drv.Client)
}

func setUp() {
	config.Set("k8s_v3", "true")
	config.InitLabels()
}

func getFakeDriver(t *testing.T) *KubernetesDriver {
	a := assert.New(t)
	config.Set("TEST", "true")
	drv, err := GetDriver()
	a.NoError(err)

	return drv
}

func TestGetEndpoint(t *testing.T) {
	setUp()
	a := assert.New(t)
	kdrv := getFakeDriver(t)
	ep := &typev1.Endpoints{
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
	kdrv.Client = fake.NewSimpleClientset(ep)

	services, err := kdrv.ListServiceEndpoints()
	a.NoError(err)
	a.Len(services, 1)
}
