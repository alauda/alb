package utils

func SliceToPointerSlice[T any](slice []T) []*T {
	pointerSlice := make([]*T, len(slice))
	for i, v := range slice {
		x := v
		pointerSlice[i] = &x
	}
	return pointerSlice
}
