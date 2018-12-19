package driver

import (
	"alb2/config"
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsfakeclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKubernetesDriver_RegisterCustomDefinedResources(t *testing.T) {
	a := assert.New(t)
	config.Set("TEST", "true")
	tests := []struct {
		name    string
		fields  []Crd
		wantErr bool
	}{
		{
			"install crds",
			CrdTypes,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := GetDriver()
			a.NoError(err)
			a.NotNil(d)
			d.ExtClient = apiextensionsfakeclient.NewSimpleClientset()

			for _, crd := range tt.fields {
				name := crd.Plural + "." + CrdGroupName
				_, err = d.ExtClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(name, metav1.GetOptions{})
				a.True(k8serrors.IsNotFound(err))
			}

			if err := d.RegisterCustomDefinedResources(); (err != nil) != tt.wantErr {
				t.Errorf("KubernetesDriver.RegisterCustomDefinedResources() error = %v, wantErr %v", err, tt.wantErr)
			}

			for _, crd := range tt.fields {
				name := crd.Plural + "." + CrdGroupName
				rv, err := d.ExtClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(name, metav1.GetOptions{})
				a.NoError(err)
				a.Equal(rv.Spec.Scope, CrdScope)
				a.Equal(rv.Spec.Names.Kind, crd.Kind)
				a.Equal(rv.Spec.Names.ListKind, crd.ListKind)
				a.Equal(rv.Spec.Names.Singular, crd.Singular)
				a.Equal(rv.Spec.Names.Plural, crd.Plural)
				a.Nil(rv.Spec.Validation)
			}
		})
	}
}
