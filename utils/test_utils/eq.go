package test_utils

import (
	"reflect"
	"sort"
)

func StringsEq(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	sort.Strings(left)
	sort.Strings(right)
	return reflect.DeepEqual(left, right)
}
