package test_utils

import "github.com/wI2L/jsondiff"

func JsonBelongsTO(left, right interface{}) (bool, interface{}, error) {
	patch, err := jsondiff.Compare(left, right)
	if err != nil {
		return false, nil, err
	}
	for _, v := range patch {
		if v.Type != "remove" {
			return false, patch, nil
		}
	}
	return true, patch, nil
}
