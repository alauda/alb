package toolkit

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func EmptyFeatureCr() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "infrastructure.alauda.io",
		Kind:    "Feature",
		Version: "v1alpha1",
	})
	return u
}

// create a new feature base on origin.
// origin will not be modified
func FeatureCr(origin *unstructured.Unstructured, name string, ns string, host string) *unstructured.Unstructured {
	// apiVersion: infrastructure.alauda.io/v1alpha1
	// kind: Feature
	// metadata:
	//   labels:
	//     instanceType: alb2
	//     type: ingress-controller
	//   name: {{ .Values.loadbalancerName }}-{{ .Values.global.namespace }}
	// spec:
	//   accessInfo:
	//     host: {{ .Values.address | quote }}
	//   instanceType: alb2
	//   type: ingress-controller
	//   version: "1.0"
	u := &unstructured.Unstructured{}
	if origin != nil {
		u = origin.DeepCopy()
	}

	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "infrastructure.alauda.io",
		Kind:    "Feature",
		Version: "v1alpha1",
	})
	u.SetName(fmt.Sprintf("%s-%s", name, ns))
	u.SetLabels(map[string]string{
		"instanceType": "alb2",
		"type":         "ingress-controller",
	})
	unstructured.SetNestedField(u.Object, map[string]interface{}{}, "spec")
	unstructured.SetNestedField(u.Object, map[string]interface{}{"host": host}, "spec", "accessInfo")
	unstructured.SetNestedField(u.Object, "alb2", "spec", "instanceType")
	unstructured.SetNestedField(u.Object, "ingress-controller", "spec", "type")
	unstructured.SetNestedField(u.Object, "1.0", "spec", "version")
	return u
}
