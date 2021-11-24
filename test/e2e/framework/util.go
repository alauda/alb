package framework

import (
	"context"
	"fmt"
	"github.com/thedevsaddam/gojsonq/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"time"
)

const (
	// Poll how often to poll for conditions
	Poll = 1 * time.Second

	// DefaultTimeout time to wait for operations to complete
	DefaultTimeout = 50 * time.Minute
)

func CreateKubeNamespace(name string, c kubernetes.Interface) (string, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	// Be robust about making the namespace creation call.
	var got *corev1.Namespace
	var err error
	err = wait.Poll(Poll, DefaultTimeout, func() (bool, error) {
		got, err = c.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		if err != nil {
			Logf("Unexpected error while creating namespace: %v", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return "", err
	}
	return got.Name, nil
}

func ToPointOfString(str string) *string {
	return &str
}

func JqFindAndTestEq(v interface{}, path string, expect string) (bool, error) {
	var ret interface{}
	switch v.(type) {
	case string:
		ret = gojsonq.New().FromString(v.(string)).Find(path)
	default:
		ret = gojsonq.New().FromInterface(v).Find(path)
	}
	retStr := fmt.Sprintf("%+v", ret)
	return retStr == expect, nil
}

func PolicyHasBackEnds(policyRaw string, ruleName string, expectBks string) bool {
	backend := gojsonq.New().
		FromString(policyRaw).
		From(`backend_group`).
		Where("name", "=", ruleName).
		Nth(1)
	bks := gojsonq.New().FromInterface(backend).Find("backends")
	bksStr := fmt.Sprintf("%v", bks)
	match := expectBks == bksStr
	Logf("compare policy %s match %v bks %s %s ", ruleName, match, expectBks, bksStr)
	return match
}

func PolicyHasRule(policyRaw string, port int, ruleName string) bool {
	rule := gojsonq.New().
		FromString(policyRaw).
		From(fmt.Sprintf("port_map.%v", port)).
		Where("rule", "=", ruleName).
		Nth(1)
	Logf("has rule port %v %v %s %v ", port, rule != nil, ruleName, rule)
	return rule != nil
}
