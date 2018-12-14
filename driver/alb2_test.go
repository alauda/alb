package driver

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/runtime"

	albfakeclient "alb2/pkg/client/clientset/versioned/fake"
	alb2scheme "alb2/pkg/client/clientset/versioned/scheme"

	"k8s.io/client-go/kubernetes/scheme"
)

func init() {
	alb2scheme.AddToScheme(scheme.Scheme)
}

func loadData(dir string) ([]runtime.Object, error) {
	var rv []runtime.Object
	decode := scheme.Codecs.UniversalDeserializer().Decode
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() || !strings.HasSuffix(path, ".json") {
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
	dataset, err := loadData("./texture")
	a.NoError(err)
	driver.ALBClient = albfakeclient.NewSimpleClientset(dataset...)
	alb, err := driver.LoadALBbyName("default", "test1")
	a.NoError(err)
	a.NotNil(alb)
}
