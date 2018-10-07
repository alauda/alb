package driver

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"
)

type fakeClient struct {
	JsonDir string
}

func (c *fakeClient) loadJson(typ, ns, name string) (string, error) {
	path := fmt.Sprintf("%s/%s-%s", c.JsonDir, typ, ns)
	if name != "" {
		path = path + "-" + name
	}
	path += ".json"

	data, err := ioutil.ReadFile(path)
	if err != nil {
		glog.Error(err)
		return "", err
	}
	return string(data), nil
}
func (c *fakeClient) Get(typ, ns, name, selector string) (string, error) {
	return c.loadJson(typ, ns, name)
}

func (c *fakeClient) Create(typ, ns, name string, resource interface{}) error {
	return nil
}
func (c *fakeClient) Update(typ, ns, name string, resource interface{}) error {
	return nil
}
func (c *fakeClient) Delete(typ, ns, name string) error {
	return nil
}

func TestLoadAlb(t *testing.T) {
	client = &fakeClient{
		JsonDir: "texture",
	}
	a := assert.New(t)
	alb, err := LoadALBbyName("default", "test1")
	a.NoError(err)
	a.NotNil(alb)
}
