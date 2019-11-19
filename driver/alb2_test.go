package driver

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	albfakeclient "alb2/pkg/client/clientset/versioned/fake"
	alb2scheme "alb2/pkg/client/clientset/versioned/scheme"

	"k8s.io/client-go/kubernetes/scheme"
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
			glog.Error(err)
			return err
		}
		obj, _, err := decode(data, nil, nil)
		if err != nil {
			glog.Error(err)
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

func TestLoadAlb(t *testing.T) {
	a := assert.New(t)
	driver, err := GetKubernetesDriver(true, 0)
	a.NoError(err)
	crdDataset, err := loadData("./texture", "crd")
	a.NoError(err)
	nativeDataset, err := loadData("./texture", "native")
	a.NoError(err)
	driver.ALBClient = albfakeclient.NewSimpleClientset(crdDataset...)
	driver.Client = fake.NewSimpleClientset(nativeDataset...)
	alb, err := driver.LoadALBbyName("default", "test1")
	a.NoError(err)
	a.NotNil(alb)
}
