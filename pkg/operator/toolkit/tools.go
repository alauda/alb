package toolkit

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/icza/dyno"
	"gopkg.in/yaml.v2"
)

func PrettyJson(data interface{}) string {
	out, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return fmt.Sprintf("err: %v could not jsonlize %v", err, data)
	}
	return string(out)
}

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

// less 1c will be 1. 600m=>1 1800m>2 2000m>2
func CpuPresetToCore(v string) int {
	// cpu limit could have value like 200m, need some calculation
	re := regexp.MustCompile(`([0-9]+)m`)
	var val int
	if string_decimal := strings.TrimRight(re.FindString(fmt.Sprintf("%v", v)), "m"); string_decimal == "" {
		val, _ = strconv.Atoi(v)
	} else {
		val_decimal, _ := strconv.Atoi(string_decimal)
		val = int(math.Ceil(float64(val_decimal) / 1000))
	}
	return val
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
