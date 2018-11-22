package ingress

import (
	"alb2/driver"
	"context"
	"flag"
	"os"
	"time"

	"testing"

	"github.com/stretchr/testify/assert"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIngres(t *testing.T) {
	a := assert.New(t)
	driver.SetDebug()
	drv, err := driver.GetDriver()
	a.NoError(err)
	a.NotNil(drv)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go MainLoop(ctx)

	time.Sleep(time.Second)
	client := drv.Client
	client.ExtensionsV1beta1().Ingresses("default").Create(
		&extsv1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		},
	)
}

func TestMain(m *testing.M) {
	flag.Set("logtostderr", "true")
	flag.Parse()
	code := m.Run()
	os.Exit(code)
}
