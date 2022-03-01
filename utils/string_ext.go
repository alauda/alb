package utils

func StrIsNillOrEq(left *string, right string) bool {
	return left == nil || (left != nil && *left == right)
}
