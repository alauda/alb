package utils

func StrIsNillOrEq(left *string, right string) bool {
	return left == nil || (left != nil && *left == right)
}

func StrListRemoveDuplicates(list []string) []string {
	m := make(map[string]string)
	for _, x := range list {
		m[x] = x
	}
	var ClearedArr []string
	for x, _ := range m {
		ClearedArr = append(ClearedArr, x)
	}
	return ClearedArr
}
