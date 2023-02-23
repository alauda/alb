package depl

import (
	"encoding/json"
	"fmt"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
)

func ShowDiff(cur, expect runtime.Object) (string, error) {
	curData, err := json.Marshal(cur)
	if err != nil {
		return "", err
	}
	expectData, err := json.Marshal(expect)
	if err != nil {
		return "", err
	}
	patchs, err := jsonpatch.CreatePatch(curData, expectData)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", patchs), nil
}
