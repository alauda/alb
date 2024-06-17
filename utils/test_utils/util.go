package test_utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"reflect"
	"syscall"
	"text/template"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kcapi "k8s.io/client-go/tools/clientcmd/api"
)

// GenCert will generate a certificate use given domain.
// you need to install openssl first.
func GenCert(domain string) (key, cert string, err error) {
	dir, err := os.MkdirTemp("", "cert")
	if err != nil {
		return "", "", err
	}
	keyPath := fmt.Sprintf("%s/key", dir)
	certPath := fmt.Sprintf("%s/cert", dir)
	shell := fmt.Sprintf(`openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes  -keyout %s -out %s -subj  /CN="%s"`, keyPath, certPath, domain)
	_, err = exec.Command("bash", "-c", shell).CombinedOutput()
	if err != nil {
		return "", "", err
	}
	keyByte, err := os.ReadFile(keyPath)
	if err != nil {
		return "", "", err
	}
	certByte, err := os.ReadFile(certPath)
	if err != nil {
		return "", "", err
	}
	os.RemoveAll(dir)
	return string(keyByte), string(certByte), nil
}

func RESTFromKubeConfig(raw string) (*rest.Config, error) {
	return clientcmd.RESTConfigFromKubeConfig([]byte(raw))
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
	kcCtx.Namespace = "default"
	kcCtx.AuthInfo = envtestName
	kubeConfig.Contexts[envtestName] = kcCtx
	kubeConfig.CurrentContext = envtestName
	contents, err := clientcmd.Write(*kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize kubeconfig file: %w", err)
	}
	return contents, nil
}

func RandomFile(base string, file string) (string, error) {
	p := path.Join(base, fmt.Sprintf("%v", rand.Int()))
	err := os.WriteFile(p, []byte(file), os.ModePerm)
	return p, err
}

func MarshOrPanic(data interface{}) string {
	out, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return string(out)
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

func CtxWithSignalAndTimeout(sec int) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(sec)*time.Second)
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		fmt.Println("rece stop signal, exit")
		cancel()
		time.Sleep(time.Second * 5)
		os.Exit(0)
	}()
	return ctx, cancel
}

func PrettyCrs[T any](objs []T) string {
	out := ""
	for _, v := range objs {
		out += PrettyCr(v) + "\n"
	}
	return out
}

func PrettyCr(obj interface{}) string {
	if obj == nil || (reflect.ValueOf(obj).Kind() == reflect.Ptr && reflect.ValueOf(obj).IsNil()) {
		return "isnill"
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	raw := map[string]interface{}{}
	err = json.Unmarshal(out, &raw)
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	{
		metadata, ok := raw["metadata"].(map[string]interface{})
		if ok {
			metadata["managedFields"] = ""
			annotation, ok := metadata["annotations"].(map[string]interface{})
			if ok {
				annotation["kubectl.kubernetes.io/last-applied-configuration"] = ""
			}
		}
	}
	out, err = json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	return string(out)
}

func WaitUtillSuccess(fn func() (bool, error)) {
	Wait(func() (bool, error) {
		defer func() {
			err := recover()
			if err != nil {
				fmt.Printf("err just rerun %v", err)
				// ret = false
			}
		}()
		return fn()
	})
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

func GinkgoAssert(e error, msg string) {
	assert.NoError(ginkgo.GinkgoT(), e, msg)
}

func GinkgoNoErr(e error) {
	assert.NoError(ginkgo.GinkgoT(), e)
}

func GinkgoAssertTrue(v bool, msg string) {
	assert.True(ginkgo.GinkgoT(), v, msg)
}
