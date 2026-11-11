package main

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/blang/semver/v4"
	"github.com/thoas/go-funk"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
	netutils "k8s.io/utils/net"
)

// isTruthy evaluates whether a value is "truthy" in template context
func isTruthy(val interface{}) bool {
	if val == nil {
		return false
	}
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Bool:
		return v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return v.Float() != 0
	case reflect.String:
		return v.String() != ""
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() > 0
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return false
		}
		return isTruthy(v.Elem().Interface())
	default:
		return true
	}
}

var funcs = template.FuncMap{
	"host": func(v string) (string, error) {
		r, err := url.Parse(v)
		if err != nil {
			return "", err
		}
		return r.Host, nil
	},
	"scheme": func(v string) (string, error) {
		r, err := url.Parse(v)
		if err != nil {
			return "", err
		}
		return r.Scheme, nil
	},
	"base64": func(v interface{}) string {
		if val, ok := v.(string); ok {
			return base64.StdEncoding.EncodeToString([]byte(val))
		}
		if val, ok := v.([]byte); ok {
			return base64.StdEncoding.EncodeToString(val)
		}
		return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%v", v)))
	},
	"md5": func(v string) string {
		r := md5.Sum([]byte(v))
		return hex.EncodeToString(r[:])
	},
	"bcrypt": func(v []byte) string {
		r, err := bcrypt.GenerateFromPassword(v, bcrypt.DefaultCost)
		if err != nil {
			return ""
		}
		return base64.StdEncoding.EncodeToString(r)
	},
	"join": func(split string, v []string) string {
		return strings.Join(v, split)
	},
	"ratio": func(ratio float64, val interface{}) string {
		var v float64
		switch val := val.(type) {
		case int:
			v = float64(val)
		case float64:
			v = val
		}
		return fmt.Sprintf("%.2f", ratio*v)
	},
	"pathJoin": func(elements ...string) string {
		return path.Join(elements...)
	},
	"quote": func(v interface{}) string {
		if v == nil {
			return `""`
		}
		if bytes, ok := v.([]byte); ok {
			return fmt.Sprintf(`"%s"`, string(bytes))
		}
		return fmt.Sprintf(`"%v"`, v)
	},
	"string": func(v interface{}) string {
		if v == nil {
			return ""
		}
		if bytes, ok := v.([]byte); ok {
			return string(bytes)
		}
		return fmt.Sprintf("%v", v)
	},
	"hasPrefix": func(s, prefix string) bool {
		return strings.HasPrefix(s, prefix)
	},
	"hasSuffix": func(s, suffix string) bool {
		return strings.HasSuffix(s, suffix)
	},
	"decodeYaml": func(s string) (map[string]interface{}, error) {
		out := map[string]interface{}{}
		return out, yaml.Unmarshal([]byte(s), &out)
	},
	"encodeYaml": func(s interface{}, indent interface{}) (string, error) {
		var ind int
		switch indent := indent.(type) {
		case int:
			ind = indent
		case float64:
			ind = int(indent)
		}
		buf := bytes.Buffer{}
		encoder := yaml.NewEncoder(&buf)
		encoder.SetIndent(ind)
		if err := encoder.Encode(s); err != nil {
			return "", err
		}
		return buf.String(), nil
	},
	"encodeJson": func(s interface{}) (string, error) {
		data, err := json.Marshal(s)
		if err != nil {
			return "", err
		}
		return string(data), nil
	},
	"list": func(objs ...interface{}) ([]interface{}, error) {
		return objs, nil
	},
	"split": func(sep, src string) []string {
		return strings.Split(src, sep)
	},
	"int": func(v interface{}) int {
		switch v := v.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			intVal, _ := strconv.Atoi(v)
			return intVal
		}
		return 0
	},
	"ref": func(s interface{}) (interface{}, error) {
		if s == nil {
			return nil, fmt.Errorf("input is nil")
		}
		val := reflect.ValueOf(s)
		if val.Kind() == reflect.Ptr {
			return val.Elem().Interface(), nil
		}
		return nil, fmt.Errorf("input %s is not a pointer", val.Kind().String())
	},
	"parseVersion": func(ver, field string) (string, error) {
		if !funk.ContainsString([]string{"Major", "Minor", "Patch", "Normalize", "String"}, field) {
			return "", fmt.Errorf("parseVersion field not valid: %s", field)
		}
		semVersion, err := semver.ParseTolerant(ver)
		if err != nil {
			return "", err
		}
		switch field {
		case "Major":
			return fmt.Sprint(semVersion.Major), nil
		case "Minor":
			return fmt.Sprint(semVersion.Minor), nil
		case "Patch":
			return fmt.Sprint(semVersion.Patch), nil
		case "Normalize":
			return fmt.Sprintf("%d.%d.%d", semVersion.Major, semVersion.Minor, semVersion.Patch), nil
		case "String":
			return semVersion.String(), nil
		}
		return "", nil
	},
	"ipv6Brackets": func(ips string, sep string) string {
		var ipsT []string
		if sep != "" {
			ipsT = strings.Split(ips, sep)
		} else {
			ipsT = []string{ips}
		}
		result := []string{}
		for _, ip := range ipsT {
			parseIP := net.ParseIP(ip)
			if parseIP.To4() == nil && parseIP.To16() != nil {
				result = append(result, fmt.Sprintf("[%s]", ip))
			} else {
				result = append(result, ip)
			}
		}
		return strings.Join(result, sep)
	},
	"getIndexedIP": func(subnet string, index int) (string, error) {
		_, svcSubnetCIDR, err := netutils.ParseCIDRSloppy(subnet)
		if err != nil {
			return "", err
		}
		dnsIP, err := netutils.GetIndexedIP(svcSubnetCIDR, index)
		if err != nil {
			return "", err
		}
		return dnsIP.String(), nil
	},
	// Mock Kubernetes functions that don't need actual client
	"labelClusterNode": func(args ...interface{}) string {
		// Mock: does nothing in test
		return ""
	},
	"lookup": func(args ...interface{}) map[string]interface{} {
		// Mock: returns empty namespace object
		return map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{},
			},
		}
	},
	"label": func(args ...interface{}) string {
		// Mock: does nothing in test
		return ""
	},
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: render <plugin-config-file> <values-file>")
		fmt.Println("Example: render ../../../deploy/chart/alb/scripts/plugin-config.yaml test-values-global.yaml")
		os.Exit(1)
	}

	pluginConfigFile := os.Args[1]
	valuesFile := os.Args[2]

	// Read values file
	valuesData, err := os.ReadFile(valuesFile)
	if err != nil {
		fmt.Printf("Error reading values file: %v\n", err)
		os.Exit(1)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(valuesData, &values); err != nil {
		fmt.Printf("Error parsing values: %v\n", err)
		os.Exit(1)
	}

	// Read plugin-config template
	pluginConfigData, err := os.ReadFile(pluginConfigFile)
	if err != nil {
		fmt.Printf("Error reading plugin-config: %v\n", err)
		os.Exit(1)
	}

	var pluginConfig map[string]interface{}
	if err := yaml.Unmarshal(pluginConfigData, &pluginConfig); err != nil {
		fmt.Printf("Error parsing plugin-config: %v\n", err)
		os.Exit(1)
	}

	// Get template content
	valuesTemplates := pluginConfig["valuesTemplates"].(map[string]interface{})
	templateContent := valuesTemplates["acp/chart-alauda-alb2"].(string)

	// Add missing template functions to funcs
	funcs["len"] = func(v interface{}) int {
		val := reflect.ValueOf(v)
		switch val.Kind() {
		case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
			return val.Len()
		default:
			return 0
		}
	}
	funcs["printf"] = fmt.Sprintf
	funcs["index"] = func(item interface{}, indices ...interface{}) interface{} {
		v := reflect.ValueOf(item)
		for _, i := range indices {
			switch v.Kind() {
			case reflect.Slice, reflect.Array:
				idx := reflect.ValueOf(i).Int()
				v = v.Index(int(idx))
			case reflect.Map:
				v = v.MapIndex(reflect.ValueOf(i))
				if !v.IsValid() {
					return nil
				}
			default:
				return nil
			}
		}
		if !v.IsValid() {
			return nil
		}
		return v.Interface()
	}
	funcs["eq"] = func(a, b interface{}) bool { return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b) }
	funcs["ne"] = func(a, b interface{}) bool { return fmt.Sprintf("%v", a) != fmt.Sprintf("%v", b) }
	funcs["and"] = func(args ...interface{}) bool {
		for _, arg := range args {
			if !isTruthy(arg) {
				return false
			}
		}
		return true
	}
	funcs["or"] = func(args ...interface{}) bool {
		for _, arg := range args {
			if isTruthy(arg) {
				return true
			}
		}
		return false
	}

	// Create template with custom functions
	tmpl, err := template.New("plugin-config").Delims("<<", ">>").Funcs(funcs).Parse(templateContent)
	if err != nil {
		fmt.Printf("Error parsing template: %v\n", err)
		os.Exit(1)
	}

	// Render template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, values); err != nil {
		fmt.Printf("Error executing template: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(buf.String())
}
