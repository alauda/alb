// Package utils: any function defined here MUST be pure function.
package utils

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"sync"
)

func CleanSyncMap(m sync.Map) {
	m.Range(func(key interface{}, value interface{}) bool {
		m.Delete(key)
		return true
	})
}

// FileExists return true if file exist and is a file.
func FileExists(filename string) (bool, error) {
	info, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

func StringRefs(s string) *string {
	return &s
}

// fix k8s issue https://github.com/kubernetes/kubernetes/issues/3030
func AddTypeInformationToObject(scheme *runtime.Scheme, obj runtime.Object) error {
	gvks, _, err := scheme.ObjectKinds(obj)
	if err != nil {
		return fmt.Errorf("missing apiVersion or kind and cannot assign it; %w", err)
	}

	for _, gvk := range gvks {
		if len(gvk.Kind) == 0 {
			continue
		}
		if len(gvk.Version) == 0 || gvk.Version == runtime.APIVersionInternal {
			continue
		}
		obj.GetObjectKind().SetGroupVersionKind(gvk)
		break
	}
	return nil
}
