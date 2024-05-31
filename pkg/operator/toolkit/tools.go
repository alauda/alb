package toolkit

import (
	"encoding/json"
	"fmt"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/icza/dyno"
	"gopkg.in/yaml.v2"
)

func PrettyCr(obj client.Object) string {
	// TODO a better way
	if IsNil(obj) {
		return "isnill"
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	raw := map[string]interface{}{}
	err = json.Unmarshal(out, &raw)
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	{
		metadata, ok := raw["metadata"].(map[string]interface{})
		if ok {
			metadata["managedFields"] = ""
			annotation, ok := metadata["annotations"].(map[string]interface{})
			if ok {
				annotation["kubectl.kubernetes.io/last-applied-configuration"] = ""
			}
		}
	}
	out, err = json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	return string(out)
}

func PrettyJsonStr(jsonstr []byte) string {
	var obj map[string]interface{}
	err := json.Unmarshal(jsonstr, &obj)
	if err != nil {
		return fmt.Sprintf("unmarshal err: %v could not jsonlize %s", err, jsonstr)
	}
	out, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return fmt.Sprintf("marshal err: %v could not jsonlize %s", err, jsonstr)
	}
	return string(out)
}

func YamlToJson(s string) (string, error) {
	var body interface{}
	err := yaml.Unmarshal([]byte(s), &body)
	if err != nil {
		return "", err
	}

	body = dyno.ConvertMapI2MapS(body)
	out, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func IsNil(i interface{}) bool {
	return i == nil || reflect.ValueOf(i).IsNil()
}

func ShowMeta(obj client.Object) string {
	if IsNil(obj) {
		return "isnil"
	}
	ns := obj.GetNamespace()
	name := obj.GetName()
	group := obj.GetObjectKind().GroupVersionKind().Group
	gversion := obj.GetObjectKind().GroupVersionKind().Version
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	version := obj.GetResourceVersion()
	return fmt.Sprintf("%s/%s/%s/%s/%s/%s", ns, name, group, gversion, kind, version)
}

func ConcatMultipleSlices[T any](slices [][]T) []T {
	var totalLen int

	for _, s := range slices {
		totalLen += len(s)
	}

	result := make([]T, totalLen)

	var i int

	for _, s := range slices {
		i += copy(result[i:], s)
	}

	return result
}
