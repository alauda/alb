package ingress

import (
	"context"
	"testing"

	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
	k8sv1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNeedEnqueueObject(t *testing.T) {

	type TestCase struct {
		description   string
		only          bool
		skip          bool
		shouldEnqueue bool
		fakeResource  test_utils.FakeResource
		ingressMeta   k8smetav1.ObjectMeta
	}

	defaultAlbs := []albv1.ALB2{
		{
			ObjectMeta: k8smetav1.ObjectMeta{
				Namespace: test_utils.DEFAULT_NS,
				Name:      "alb-1",
			},
		},
	}
	testCases := []TestCase{
		{
			description:   "do not enqueue when annotations['kubernetes.io/ingress.class'] is not empty and not alb name",
			shouldEnqueue: false,
			fakeResource: test_utils.FakeResource{
				Alb: test_utils.FakeALBResource{
					Albs: defaultAlbs,
				},
			},
			ingressMeta: k8smetav1.ObjectMeta{
				Name:        "ingress1",
				Annotations: map[string]string{"kubernetes.io/ingress.class": "x"},
			},
		},
		{
			description:   "enqueue when alb be assign to all project",
			shouldEnqueue: true,

			fakeResource: test_utils.FakeResource{
				Alb: test_utils.FakeALBResource{
					Albs: []albv1.ALB2{
						{
							ObjectMeta: k8smetav1.ObjectMeta{
								Namespace: test_utils.DEFAULT_NS,
								Name:      "alb-1",
								Labels: map[string]string{
									"project.alauda.io/name":    "project-1",
									"project.alauda.io/ALL_ALL": "true",
								},
							},
						},
					},
				},
				K8s: test_utils.FakeK8sResource{
					Namespaces: []k8sv1.Namespace{
						{
							ObjectMeta: k8smetav1.ObjectMeta{
								Name:   test_utils.DEFAULT_NS,
								Labels: map[string]string{"alauda.io/project": "random-project"},
							},
						},
					},
				},
			},
			ingressMeta: k8smetav1.ObjectMeta{Name: "ingress1"},
		},
		// TODO add more case here
		// 1. enqueue when alb role is instance and ingress's project belongs to ft's project
		// 2. enqueue when alb role is port and ingress's project belongs to alb's project
		// 3. do not lenqueue when alb role is port,without project and ingress without project either
	}

	// filter the testcase we what to run
	runCases := func(testCases []TestCase) []TestCase {
		runCases := make([]TestCase, 0)
		hasOnly := false
		// if there is only we just pick this only
		for _, testCase := range testCases {
			if testCase.only {
				hasOnly = true
				break
			}
		}
		for _, testCase := range testCases {
			if !hasOnly {
				if !testCase.skip {
					runCases = append(runCases, testCase)
				}
			} else {
				if testCase.only && !testCase.skip {
					runCases = append(runCases, testCase)
				}
			}
		}

		return runCases
	}(testCases)

	for index, testCase := range runCases {
		t.Logf("case %d: %s\n", index, testCase.description)

		ctx, cancel := context.WithCancel(context.Background())
		a := assert.New(t)
		defer cancel()
		drv, informers := test_utils.InitFakeAlb(t, ctx, testCase.fakeResource, test_utils.DEFAULT_CONFIG_FOR_TEST)

		ingressController := NewController(drv, informers.Alb.Alb, informers.Alb.Rule, informers.K8s.Ingress, informers.K8s.Namespace.Lister())
		need := ingressController.needEnqueueObject(&testCase.ingressMeta)
		a.Equal(need, testCase.shouldEnqueue, testCase.description)

	}
}
