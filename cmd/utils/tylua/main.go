package main

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	ct "alauda.io/alb2/controller/types"
	"golang.org/x/exp/maps"
)

type LuaHint struct {
	Kind        string // struct or alias
	Fields      map[string]string
	EmbedFields []string
	Alias       string
	Name        string
	Order       int
}

type LuaHintMap map[string]LuaHint

func main() {
	hm := LuaHintMap{}
	t := reflect.TypeFor[ct.NgxPolicy]()
	resolve(t, hm, "", 0)
	hints := maps.Values(hm)
	sort.Slice(hints, func(i, j int) bool {
		if hints[i].Order != hints[j].Order {
			return hints[i].Order < hints[j].Order
		}
		return hints[i].Name < hints[j].Name
	})
	s := ""
	s += "--- @alias CJSON_NULL userdata\n"
	for _, h := range hints {
		if h.Kind == "struct" {
			s += fmt.Sprintf("--- @class %s\n", h.Name)
			fs := maps.Keys(h.Fields)
			sort.Strings(fs)
			for _, k := range fs {
				v := h.Fields[k]
				s += fmt.Sprintf("--- @field %s %s\n", k, v)
			}
			for _, v := range h.EmbedFields {
				s += resolve_embed(hm, v)
			}
			s += "\n\n"
		}
	}
	fmt.Printf("-----")
	fmt.Printf("%s", s)
	fmt.Printf("write to %s", os.Args[1])
	_ = os.WriteFile(os.Args[1], []byte(s), 0o644)
}

func resolve_embed(hm LuaHintMap, key string) string {
	h := hm[key]
	s := ""
	if h.Kind == "struct" {
		fs := maps.Keys(h.Fields)
		sort.Strings(fs)
		for _, k := range fs {
			v := h.Fields[k]
			s += fmt.Sprintf("--- @field %s %s\n", k, v)
		}
		ef := h.EmbedFields
		sort.Strings(ef)
		for _, v := range ef {
			s += resolve_embed(hm, v)
		}
	}
	return s
}

func resolve(f reflect.Type, hm LuaHintMap, tag reflect.StructTag, order int) string {
	fmt.Println("resolve type", f.Kind(), f.Name())
	omit_empty := strings.Contains(tag.Get("json"), "omitempty")
	or_empty := func(s string, empty bool) string {
		if empty {
			return fmt.Sprintf("%s?", s)
		}
		return fmt.Sprintf("(%s|CJSON_NULL)", s)
	}
	if f.Kind() == reflect.Map {
		k := resolve(f.Key(), hm, "", order+1)
		v := resolve(f.Elem(), hm, "", order+1)
		return fmt.Sprintf("table<%s, %s>", k, v)
	}
	if f.Kind() == reflect.Slice {
		k := resolve(f.Elem(), hm, "", order+1)
		if f.Elem().Kind() == reflect.Ptr {
			k = resolve(f.Elem().Elem(), hm, "", order+1)
		}
		return fmt.Sprintf("%s[]", k)
	}
	if f.Kind() == reflect.Ptr {
		fmt.Println("resolve ptr", f)
		k := resolve(f.Elem(), hm, "", order+1)
		return or_empty(k, omit_empty)
	}
	if f.Kind() == reflect.Array {
		k := resolve(f.Elem(), hm, "", order+1)
		return fmt.Sprintf("%s[]", k)
	}
	if f.Kind() == reflect.Struct {
		name := f.Name()
		hm[name] = LuaHint{
			Kind:        "struct",
			Alias:       "",
			Order:       order,
			Fields:      map[string]string{},
			EmbedFields: []string{},
			Name:        name,
		}
		for i := 0; i < f.NumField(); i++ {
			ff := f.Field(i)
			fmt.Println("handle field", ff.Name, ff.Type, ff.Anonymous)

			field_name := strings.Split(ff.Tag.Get("json"), ",")[0]
			if field_name == "-" {
				continue
			}
			if ff.Anonymous {
				hint := resolve(ff.Type, hm, ff.Tag, order+1)
				lh := hm[name]
				lh.EmbedFields = append(lh.EmbedFields, hint)
				hm[name] = lh
				continue
			}
			hint := resolve(ff.Type, hm, ff.Tag, order+1)
			hm[name].Fields[field_name] = hint
		}
		return name
	}
	return default_lua_type(f.Kind().String())
}

func default_lua_type(s string) string {
	_, ret := default_type_map(s)
	return ret
}

func default_type_map(s string) (bool, string) {
	switch s {
	case "interface":
		return true, "any"
	case "bool":
		return true, "boolean"
	case "string":
		return true, "string"
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64",
		"complex64", "complex128":
		return true, "number"
	}
	return false, s
}
