package framework

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	ct "alauda.io/alb2/controller/types"
	tu "alauda.io/alb2/utils/test_utils"
	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	"github.com/thedevsaddam/gojsonq/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// Poll how often to poll for conditions
	Poll = 1 * time.Second

	// DefaultTimeout time to wait for operations to complete
	DefaultTimeout = 50 * time.Minute
)

func ToPointOfString(str string) *string {
	return &str
}

func JqFindAndTestEq(v interface{}, path string, expect string) (bool, error) {
	var ret interface{}
	switch v := v.(type) {
	case string:
		ret = gojsonq.New().FromString(v).Find(path)
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
		From(fmt.Sprintf("http.tcp.%v", port)).
		Where("rule", "=", ruleName).
		Nth(1)
	Logf("has rule port %v %v %s %v ", port, rule != nil, ruleName, rule)
	return rule != nil
}

func GIt(text string, body interface{}, timeout ...float64) bool {
	return ginkgo.It("alb-test-case "+text, body, timeout...)
}

func GFIt(text string, body interface{}, timeout ...float64) bool {
	if os.Getenv("ALB_IGNORE_FOCUS") == "true" {
		return GIt(text, body, timeout...)
	}
	return ginkgo.FIt("alb-test-case "+text, body, timeout...)
}

func listen(network, addr string, stopCh chan struct{}) {
	go func() {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			fmt.Println(err.Error())
		}
		<-stopCh
		listener.Close()
	}()
}

func ListenTcp(port string, stopCh chan struct{}) {
	listen("tcp", ":"+port, stopCh)
}

func ListenUdp(port string, stopCh chan struct{}) {
	listen("udp", ":"+port, stopCh)
}

func Access(f func()) {
	defer func() {
		recover()
	}()
	f()
}

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

// Logf logs to the INFO logs.
// Deprecated: use GinkgoLog instead.
func Logf(format string, args ...interface{}) {
	fmt.Printf(nowStamp()+": "+"info"+": "+"envtest framework : "+format+"\n", args...)
}

func CfgFromFile(p string) *rest.Config {
	Logf("cfg from file %v", p)
	cf, err := clientcmd.BuildConfigFromFlags("", p)
	if err != nil {
		panic(err)
	}
	return cf
}

func GAssert[T any](f func() (T, error), msg string) T {
	ret, err := f()
	assert.NoError(ginkgo.GinkgoT(), err, msg)
	return ret
}

func BuildBG(name, mode string, bs ct.Backends) ct.BackendGroup {
	return ct.BackendGroup{
		Name:     name,
		Mode:     mode,
		Backends: bs,
	}
}

func TestEq(f func() bool, msg ...string) (ret bool) {
	defer func() {
		err := recover()
		if err != nil {
			Logf("TestEq %s err: %+v", msg, err)
			ret = false
		}
	}()
	ret = f()
	Logf("TestEq %s  %v", msg, ret)
	return ret
}

func CreateToken(kc *tu.Kubectl, name string, ns string) (string, error) {
	cmds := []string{
		"create", "token", name,
	}
	if ns != "" {
		cmds = append(cmds, "-n", ns)
	}
	out, err := kc.Kubectl(cmds...)
	fmt.Printf("out %v\n", out)
	if err != nil {
		return "", err
	}
	return out, nil
}

func CreateRestCfg(origin *rest.Config, token string) *rest.Config {
	return &rest.Config{
		Host:        origin.Host,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
}

func MakeDeploymentReady(ctx context.Context, cli kubernetes.Interface, ns, name string) {
	for {
		time.Sleep(time.Second * 1)
		dep, err := cli.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		dep.Status.ReadyReplicas = *dep.Spec.Replicas
		dep.Status.Replicas = *dep.Spec.Replicas
		_, err = cli.AppsV1().Deployments(ns).UpdateStatus(ctx, dep, metav1.UpdateOptions{})
		if err == nil {
			break
		}
		fmt.Println("ok exit")
	}
}
