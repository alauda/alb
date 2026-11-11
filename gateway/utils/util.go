package utils

import (
	"fmt"

	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func RefToKey(ref gv1.ParentReference, ns string) string {
	if ref.Namespace != nil {
		ns = string(*ref.Namespace)
	}
	section_name := ""
	if ref.SectionName != nil {
		section_name = string(*ref.SectionName)
	}
	return fmt.Sprintf("%s/%s/%s", ns, ref.Name, section_name)
}
