package driver

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "alauda.io/alb2/config"
	albfakeclient "alauda.io/alb2/pkg/client/clientset/versioned/fake"
	alb2scheme "alauda.io/alb2/pkg/client/clientset/versioned/scheme"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
)

func init() {
	alb2scheme.AddToScheme(scheme.Scheme)
	corev1.AddToScheme(scheme.Scheme)
}

func loadData(dir, prefix string) ([]runtime.Object, error) {
	var rv []runtime.Object
	decode := scheme.Codecs.UniversalDeserializer().Decode
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		if !strings.HasPrefix(filepath.Base(path), prefix) {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			klog.Error(err)
			return err
		}
		obj, _, err := decode(data, nil, nil)
		if err != nil {
			klog.Error(err)
			return err
		}
		rv = append(rv, obj)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return rv, nil
}

// TODO use envtest to test LoadALBbyName
func TestLoadAlb(t *testing.T) {
	t.SkipNow()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	os.Setenv("DOMAIN", "alauda.io")
	// TODO fix me
	// config.Set("TWEAK_DIRECTORY", "./texture") // set TWEAK_DIRECTORY to a exist path, make calculate hash happy

	a := assert.New(t)
	driver, err := GetKubernetesDriver(ctx, true)
	a.NoError(err)
	crdDataset, err := loadData("./texture", "crd")
	a.NoError(err)
	nativeDataset, err := loadData("./texture", "native")
	a.NoError(err)
	driver.ALBClient = albfakeclient.NewSimpleClientset(crdDataset...)
	driver.Client = fake.NewSimpleClientset(nativeDataset...)
	InitDriver(driver, ctx)

	alb, err := driver.LoadALBbyName("default", "test1")
	a.NoError(err)
	a.NotNil(alb)
}
