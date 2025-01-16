package utils

import (
	"fmt"
	"reflect"
	"strings"
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

type ReassignStructOpt struct {
	Resolver map[string]func(l reflect.Value, r reflect.Value, root reflect.Value) error
}

func ReassignStructViaMapping(left interface{}, right interface{}, opt ReassignStructOpt) error {
	left_key_v_field_map := map[string]reflect.Value{}
	left_key_t_field_map := map[string]reflect.StructField{}
	left_type := reflect.TypeOf(left).Elem()
	left_val := reflect.ValueOf(left).Elem()
	right_type := reflect.TypeOf(right).Elem()
	right_val := reflect.ValueOf(right).Elem()

	for i := 0; i < left_type.NumField(); i++ {
		left_field_type := left_type.Field(i)
		left_field_val := left_val.Field(i)
		if left_field_val.Kind() == reflect.Struct && left_field_type.Anonymous {
			_ = ReassignStructViaMapping(left_field_val.Addr().Interface(), right, opt)
			continue
		}
		tag := left_field_type.Tag.Get("key")
		if tag == "" {
			continue
		}
		left_key_v_field_map[tag] = left_field_val
		left_key_t_field_map[tag] = left_field_type
	}

	for i := 0; i < right_type.NumField(); i++ {
		right_field_type := right_type.Field(i)
		right_field_val := right_val.Field(i)

		if right_field_val.Kind() == reflect.Struct && right_field_type.Anonymous {
			_ = ReassignStructViaMapping(left, right_field_val.Addr().Interface(), opt)
			continue
		}
		tag := right_field_type.Tag.Get("key")
		if tag == "" {
			continue
		}
		trans := right_field_type.Tag.Get("trans")
		if _, ok := left_key_v_field_map[tag]; !ok {
			continue
		}
		left_field_val := left_key_v_field_map[tag]
		left_field_type := left_key_t_field_map[tag]
		if !(left_field_val.IsValid() && right_field_val.CanSet()) {
			continue
		}
		if left_field_val.Type() == right_field_val.Type() && trans == "" {
			if (left_field_val.Kind() == reflect.Ptr || left_field_val.Kind() == reflect.Slice || left_field_val.Kind() == reflect.Map) && left_field_val.IsNil() {
				continue
			}
			right_field_val.Set(left_field_val)
			continue
		}
		if left_field_val.Type().Kind() == reflect.String && right_field_val.Kind() == reflect.Bool && trans == "from_bool" {
			right_field_val.SetBool(strings.ToLower(left_field_val.String()) == "true")
			continue
		}
		if resolver, ok := opt.Resolver[trans]; ok {
			err := resolver(left_field_val, right_field_val, reflect.ValueOf(right))
			if err != nil {
				return err
			}
			continue
		}
		return fmt.Errorf("not support type trans %s on key %v from %v to %v", trans, tag, left_field_type.Name, right_field_type.Name)
	}
	return nil
}
