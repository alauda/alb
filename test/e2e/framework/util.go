package framework

import (
	"context"
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
