package framework

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"

	ct "alauda.io/alb2/controller/types"
	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	"github.com/thedevsaddam/gojsonq/v2"
	"k8s.io/apimachinery/pkg/util/wait"
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

func GinkgoAssert(e error, msg string) {
	assert.NoError(ginkgo.GinkgoT(), e, msg)
}
func GinkgoNoErr(e error) {
	assert.NoError(ginkgo.GinkgoT(), e)
}

func GinkgoAssertTrue(v bool, msg string) {
	assert.True(ginkgo.GinkgoT(), v, msg)
}

func random() string {
	return fmt.Sprintf("%v", rand.Int())
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

func log(level string, format string, args ...interface{}) {
	fmt.Fprintf(ginkgo.GinkgoWriter, nowStamp()+": "+level+": "+"envtest framework : "+format+"\n", args...)
}

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

// Logf logs to the INFO logs.
// Deprecated: use GinkgoLog instead.
func Logf(format string, args ...interface{}) {
	log("INFO", format, args...)
}

func CfgFromFile(p string) *rest.Config {
	Logf("cfg from file %v", p)
	cf, err := clientcmd.BuildConfigFromFlags("", p)
	if err != nil {
		panic(err)
	}
	return cf
}

// TODO 废弃掉这种方法，应该使用ginkgo测试中的全局变量
func CfgFromEnv() *rest.Config {
	kubecfg := os.Getenv("KUBECONFIG")
	return CfgFromFile(kubecfg)
}

func Wait(fn func() (bool, error)) {
	const (
		// Poll how often to poll for conditions
		Poll = 1 * time.Second

		// DefaultTimeout time to wait for operations to complete
		DefaultTimeout = 50 * time.Minute
	)
	err := wait.Poll(Poll, DefaultTimeout, fn)
	assert.Nil(ginkgo.GinkgoT(), err, "wait fail")
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
