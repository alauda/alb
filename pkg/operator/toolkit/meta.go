package toolkit

import (
	"strings"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FmtKeyBySep is return string "key1+sep+key2"
func FmtKeyBySep(sep string, key ...string) string {
	return strings.Join(key, sep)
}

func MakeOwnerRefs(alb2 *albv2.ALB2) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: albv2.SchemeGroupVersion.String(),
			Kind:       albv2.ALB2Kind,
			Name:       alb2.GetName(),
			UID:        alb2.GetUID(),
		},
	}
}
