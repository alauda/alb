// Package utils: any function defined here MUST be pure function.
package utils

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
)

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

func RandomStr(prefix string, length int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	const ALPHANUM = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = ALPHANUM[r.Intn(len(ALPHANUM))]
	}
	if prefix != "" {
		return prefix + "-" + string(result)
	}
	return string(result)
}

func PrettyJson(data interface{}) string {
	out, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return fmt.Sprintf("err: %v could not jsonlize %v", err, data)
	}
	return string(out)
}
