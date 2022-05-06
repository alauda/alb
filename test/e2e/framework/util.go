package framework

import (
	"bytes"
	"fmt"
	"html/template"
	"math/rand"
	"net"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/thedevsaddam/gojsonq/v2"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kcapi "k8s.io/client-go/tools/clientcmd/api"
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

func KubeConfigFromREST(cfg *rest.Config, envtestName string) ([]byte, error) {
	kubeConfig := kcapi.NewConfig()
	protocol := "https"
	if !rest.IsConfigTransportTLS(*cfg) {
		protocol = "http"
	}

	// cfg.Host is a URL, so we need to parse it so we can properly append the API path
	baseURL, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("unable to interpret config's host value as a URL: %w", err)
	}

	kubeConfig.Clusters[envtestName] = &kcapi.Cluster{
		// TODO(directxman12): if client-go ever decides to expose defaultServerUrlFor(config),
		// we can just use that.  Note that this is not the same as the public DefaultServerURL,
		// which requires us to pass a bunch of stuff in manually.
		Server:                   (&url.URL{Scheme: protocol, Host: baseURL.Host, Path: cfg.APIPath}).String(),
		CertificateAuthorityData: cfg.CAData,
	}
	kubeConfig.AuthInfos[envtestName] = &kcapi.AuthInfo{
		// try to cover all auth strategies that aren't plugins
		ClientCertificateData: cfg.CertData,
		ClientKeyData:         cfg.KeyData,
		Token:                 cfg.BearerToken,
		Username:              cfg.Username,
		Password:              cfg.Password,
	}
	kcCtx := kcapi.NewContext()
	kcCtx.Cluster = envtestName
	kcCtx.AuthInfo = envtestName
	kubeConfig.Contexts[envtestName] = kcCtx
	kubeConfig.CurrentContext = envtestName

	contents, err := clientcmd.Write(*kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize kubeconfig file: %w", err)
	}
	return contents, nil
}

func Template(templateStr string, data map[string]interface{}) string {
	buf := new(bytes.Buffer)
	t, err := template.New("s").Parse(templateStr)
	if err != nil {
		panic(err)
	}
	err = t.Execute(buf, data)
	if err != nil {
		panic(err)
	}
	return buf.String()
}

func Kubectl(options ...string) (string, error) {
	cmd := exec.Command("kubectl", options...)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s err: %v", stdout, err)
	}
	return string(stdout), nil
}

func Access(f func()) {
	defer func() {
		recover()
	}()
	f()
}

func TestEq(f func() bool) (ret bool) {
	defer func() {
		err := recover()
		if err != nil {
			Logf("TestEq err: %+v", err)
			ret = false
		}
	}()
	return f()
}
