package utils

import (
	"fmt"
	"reflect"
)

type ResolveAnnotationOpt struct {
	Prefix []string // 我们支持多个前缀，这样用户可以用我们自己的前缀来覆盖掉nginx的前缀，这样能保证他们的兼容性
}

func ResolverStructFromAnnotation(t interface{}, annotation map[string]string, opt ResolveAnnotationOpt) error {
	t_type := reflect.TypeOf(t).Elem()
	t_val := reflect.ValueOf(t).Elem()
	for i := 0; i < t_type.NumField(); i++ {
		t_field := t_type.Field(i)
		v_field := t_val.Field(i)
		if v_field.Kind() == reflect.Struct && t_field.Anonymous {
			_ = ResolverStructFromAnnotation(v_field.Addr().Interface(), annotation, opt)
		}

		tag := t_field.Tag.Get("annotation")
		if tag == "" {
			continue
		}
		default_val := t_field.Tag.Get("default")
		// we only support string field to set
		if !(v_field.CanSet() && v_field.Kind() == reflect.String) {
			continue
		}
		resolved := false
		for _, prefix := range opt.Prefix {
			full_key := fmt.Sprintf("%s/%s", prefix, tag)
			if value, ok := annotation[full_key]; ok {
				v_field.SetString(value)
				resolved = true
				break
			}
		}
		if !resolved {
			v_field.SetString(default_val)
		}
	}
	return nil
}
