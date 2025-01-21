package main

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/template"

	. "alauda.io/alb2/pkg/controller/ext/auth/types"
	"github.com/kr/pretty"
)

func main() {
	type MapCfg struct {
		base    string
		mapping []struct {
			from reflect.Type
			to   reflect.Type
		}
	}
	cfg := MapCfg{
		base: "./pkg/controller/ext/auth/types/",
		mapping: []struct {
			from reflect.Type
			to   reflect.Type
		}{
			{
				from: reflect.TypeOf((*AuthIngressForward)(nil)).Elem(),
				to:   reflect.TypeOf((*ForwardAuthInCr)(nil)).Elem(),
			},
			{
				from: reflect.TypeOf((*ForwardAuthInCr)(nil)).Elem(),
				to:   reflect.TypeOf((*ForwardAuthPolicy)(nil)).Elem(),
			},
			{
				from: reflect.TypeOf((*AuthIngressBasic)(nil)).Elem(),
				to:   reflect.TypeOf((*BasicAuthInCr)(nil)).Elem(),
			},
			{
				from: reflect.TypeOf((*BasicAuthInCr)(nil)).Elem(),
				to:   reflect.TypeOf((*BasicAuthPolicy)(nil)).Elem(),
			},
		},
	}
	for _, m := range cfg.mapping {
		lt := m.from
		rt := m.to
		base := cfg.base
		f := fmt.Sprintf(base+"codegen_mapping_%s_%s.go", strings.ToLower(lt.Name()), strings.ToLower(rt.Name()))
		out, err := trans(lt, rt)
		if err != nil {
			panic(err)
		}
		err = os.WriteFile(f, []byte(out), 0o644)
		if err != nil {
			panic(err)
		}
	}
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func find_field_via_key(t reflect.Type, key string) *reflect.StructField {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Tag.Get("key") == key {
			return &f
		}
	}
	return nil
}

func field_type(v reflect.Type) string {
	if v.Kind() == reflect.Ptr ||
		v.Kind() == reflect.Interface ||
		v.Kind() == reflect.Slice ||
		v.Kind() == reflect.Map ||
		v.Kind() == reflect.Chan ||
		v.Kind() == reflect.Func {
		return "pointer"
	}
	if v.Kind() == reflect.String {
		return "string"
	}
	return "other"
}

func trans(lt reflect.Type, rt reflect.Type) (string, error) {
	TEMPLATE := `
package {{.pkg}}

import (
	"strings"
)

func init() {
	// make go happy
	_ = strings.Clone("")
}

type ReAssign{{.from}}To{{.to}}Opt struct {
    {{ range $trans_name, $trans_cfg := .trans_map }}
    {{$trans_name}} func({{$trans_cfg.trans_from_type}}) ({{$trans_cfg.trans_to_type}}, error)
    {{end}}
}

var ReAssign{{.from}}To{{.to}}Trans = map[string]func(lt *{{.from}}, rt *{{.to}}, opt *ReAssign{{.from}}To{{.to}}Opt) error{
           {{ range $name, $field_cfg := .field_map }}
                  {{ if $field_cfg.trans_name  -}}
                  "{{$name}}": func(lt *{{$.from}}, rt *{{$.to}}, opt *ReAssign{{$.from}}To{{$.to}}Opt) error {
                     {{if eq $field_cfg.trans_name "From_bool"}}
                      ret := strings.ToLower(lt{{$field_cfg.l_access}}) == "true"
                      rt{{$field_cfg.r_access}} = ret
                      return nil
                     {{else}}
                      ret, err := opt.{{$field_cfg.trans_name}}(lt{{$field_cfg.l_access}})
                      if err != nil {
                          return err
                      }
                      rt{{$field_cfg.r_access}} = ret
                      return nil
                     {{end}}
                  },
                  {{ end }}
           {{end}}
}

func ReAssign{{.from}}To{{.to}}(lt *{{.from}}, rt *{{.to}}, opt *ReAssign{{.from}}To{{.to}}Opt) error {
    {{ range $name, $field_cfg := .field_map }}
    {{ if not $field_cfg.trans_name  -}}
		{{ if eq $field_cfg.type "pointer" }}
    	if lt{{$field_cfg.l_access}} != nil {
          rt{{$field_cfg.r_access}} = lt{{$field_cfg.l_access}}
    	}
        {{ else if eq $field_cfg.type "string" }}
        if lt{{$field_cfg.l_access}} != "" {
          rt{{$field_cfg.r_access}} = lt{{$field_cfg.l_access}}
        }
        {{ else }}
        rt{{$field_cfg.r_access}} = lt{{$field_cfg.l_access}}
        {{ end }}
    {{- end -}}
    {{- end }}
    for _, m := range ReAssign{{.from}}To{{.to}}Trans {
            err := m(lt, rt, opt)
            if err != nil {
                    return err
            }
    }
    return nil
}
`
	pkg_path := strings.Split(lt.PkgPath(), "/")
	pkg := pkg_path[len(pkg_path)-1]
	// 准备模板数据
	data := map[string]interface{}{
		"from":      lt.Name(),
		"pkg":       pkg,
		"to":        rt.Name(),
		"field_map": make(map[string]map[string]string),
		"trans_map": make(map[string]map[string]string),
	}

	fieldMap := make(map[string]map[string]string)
	transMap := make(map[string]map[string]string)

	// 从右侧类型开始遍历
	for i := 0; i < rt.NumField(); i++ {
		rf := rt.Field(i)
		key := rf.Tag.Get("key")
		if key == "" {
			continue
		}
		lf_or_null := find_field_via_key(lt, key)
		if lf_or_null == nil {
			return "", fmt.Errorf("%s key not find in left type", key)
		}
		lf := *lf_or_null
		// 在左侧类型中查找具有相同key的字段
		fieldCfg := make(map[string]string)
		fieldCfg["l_access"] = "." + lf.Name
		fieldCfg["r_access"] = "." + rf.Name
		fieldCfg["type"] = field_type(rf.Type)

		// 检查是否需要特殊转换
		trans := upperFirst(rf.Tag.Get("trans"))

		if trans != "" {
			if trans == "from_bool" {
				continue
			}
			fieldCfg["trans_name"] = trans
			// 添加到转换函数映射
			transCfg := make(map[string]string)
			transCfg["trans_from_type"] = lf.Type.String()
			transCfg["trans_to_type"] = strings.ReplaceAll(rf.Type.String(), pkg+".", "")
			fmt.Printf("%s xx %v\n", trans, rf.Type.String())
			transMap[trans] = transCfg
			// Check for duplicate trans names
			if t, exists := transMap[trans]; exists && !reflect.DeepEqual(t, transCfg) {
				return "", fmt.Errorf("trans name '%s' %v %v fConflict", trans, t, transCfg)
			}
			fieldMap[key] = fieldCfg
			continue
		}

		if lf.Type == rf.Type {
			fieldMap[key] = fieldCfg
			continue
		}
		return "", fmt.Errorf("could not find trans from %s to %s for %s", lf.Type.String(), rf.Type.String(), key)
	}

	data["field_map"] = fieldMap
	data["trans_map"] = transMap

	tmpl, err := template.New("trans").Parse(TEMPLATE)
	if err != nil {
		return "", err
	}

	pretty.Print(data)
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
