package utils

import (
	"strings"

	"github.com/samber/lo"
)

func StrIsNillOrEq(left *string, right string) bool {
	return left == nil || (left != nil && *left == right)
}

func StrListRemoveDuplicates(list []string) []string {
	m := make(map[string]string)
	for _, x := range list {
		m[x] = x
	}
	var ClearedArr []string
	for x := range m {
		ClearedArr = append(ClearedArr, x)
	}
	return ClearedArr
}

func SplitAndRemoveEmpty(s string, sep string) []string {
	items := strings.Split(s, sep)
	return lo.Filter(items, func(s string, _ int) bool { return strings.TrimSpace(s) != "" })
}
