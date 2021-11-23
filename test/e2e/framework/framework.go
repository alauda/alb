package framework

import (
	albCtl "alauda.io/alb2/alb"
	m "alauda.io/alb2/modules"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albclient "alauda.io/alb2/pkg/client/clientset/versioned"
	"context"
	"fmt"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

type Framework struct {
	k8sClient    kubernetes.Interface
	albClient    albclient.Interface
	cfg          *rest.Config
	ctx          context.Context
	cancel       func()
	namespace    string
	albName      string
	baseDir      string
	nginxCfgPath string
	policyPath   string
	albLogPath   string
	domain       string
	defaultFt    int
}

type Config struct {
	RandomBaseDir bool
	RestCfg       *rest.Config
}

func CfgFromEnv() *rest.Config {
	host := os.Getenv("KUBERNETES_SERVER")

	Expect(host).ShouldNot(BeEmpty())
	return &rest.Config{
		Host: host,
	}
}

func EnvTestCfgToEnv(cfg *rest.Config) {
	os.Setenv("KUBERNETES_SERVER", "http://"+cfg.Host)
}

func NewAlb(ext Config) *Framework {
	cfg := ext.RestCfg
	var baseDir = os.TempDir() + "/alb-e2e-test"
	if ext.RandomBaseDir {
		var err error
		baseDir, err = os.MkdirTemp("", "alb-e2e-test")
		assert.Nil(ginkgo.GinkgoT(), err, "creat temp dir")
	} else {
		os.RemoveAll(baseDir)
		os.MkdirAll(baseDir, os.ModePerm)
	}

	Logf("base dir %v", baseDir)
	name := "alb-dev"
	domain := "cpaas.io"
	ns := "cpaas-system"
	nginxCfgPath := baseDir + "/nginx.conf"
	nginxPolicyPath := baseDir + "/policy.new"
	os.WriteFile(nginxCfgPath, []byte(""), os.ModePerm) // give it a default empty nginx.conf
	Logf("apiserver %s", cfg.Host)
	os.Setenv("KUBERNETES_SERVER", cfg.Host)
	os.Setenv("KUBERNETES_BEARERTOKEN", cfg.BearerToken)
	os.Setenv("NAME", name)
	os.Setenv("NAMESPACE", ns)
	os.Setenv("DOMAIN", domain)
	os.Setenv("ALB_ROTATE_LOG", "false")
	os.Setenv("NEW_CONFIG_PATH", nginxCfgPath+".new")
	os.Setenv("OLD_CONFIG_PATH", nginxCfgPath)
	os.Setenv("NEW_POLICY_PATH", nginxPolicyPath)
	os.Setenv("ALB_E2E_TEST_CONTROLLER_ONLY", "true")
	twekDir := baseDir + "/tweak"
	os.MkdirAll(twekDir, os.ModePerm)
	os.Setenv("ALB_TWEAK_DIRECTORY", twekDir)

	cwd, err := os.Getwd()
	assert.Nil(ginkgo.GinkgoT(), err, "get cwd")
	nginxTemplatePath, err := filepath.Abs(filepath.Join(cwd, "../../template/nginx/nginx.tmpl"))
	assert.Nil(ginkgo.GinkgoT(), err, "nginx template")
	assert.FileExists(ginkgo.GinkgoT(), nginxTemplatePath, "nginx template")
	os.Setenv("NGINX_TEMPLATE_PATH", nginxTemplatePath)
	os.Setenv("INTERVAL", "5")
	os.Setenv("ALB_RELOAD_TIMEOUT", "5") // 5s

	statusDir := baseDir + "/last_status"
	os.Setenv("ALB_STATUSFILE_PARENTPATH", statusDir)
	os.MkdirAll(statusDir, os.ModePerm)

	os.Setenv("ALB_LOG_EXT", "true")
	alblogpath := baseDir + "/alb.log"
	os.Setenv("ALB_LOG_FILE", alblogpath)
	os.Setenv("ALB_DISABLE_LOG_STDERR", "true")

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Framework{
		baseDir:      baseDir,
		cfg:          cfg,
		k8sClient:    k8sClient,
		albClient:    albclient.NewForConfigOrDie(cfg),
		nginxCfgPath: nginxCfgPath,
		policyPath:   nginxPolicyPath,
		albLogPath:   alblogpath,
		ctx:          ctx,
		cancel:       cancel,
		namespace:    ns,
		albName:      name,
		domain:       domain,
		defaultFt:    12345,
	}
}

func (f *Framework) Init() {
	_, err := CreateKubeNamespace(f.namespace, f.k8sClient)
	assert.Nil(ginkgo.GinkgoT(), err, "creating ns")

	// create alb
	alb, err := f.albClient.CrdV1().ALB2s(f.namespace).Create(f.ctx, &alb2v1.ALB2{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.namespace,
			Name:      f.albName,
		},
		Spec: alb2v1.ALB2Spec{
			Domains: []string{},
		},
	}, metav1.CreateOptions{})
	assert.Nil(ginkgo.GinkgoT(), err, "creating alb")
	Logf("create alb success %s/%s", alb.Namespace, alb.Name)

	// create ft, this default port is meaningless, just used to make sure alb running healthily
	ft, err := f.albClient.CrdV1().Frontends(f.namespace).Create(f.ctx, &alb2v1.Frontend{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.namespace,
			Name:      fmt.Sprintf("%s-%05d", f.albName, f.defaultFt),
			// the most import part
			Labels: map[string]string{
				fmt.Sprintf("alb2.%s/name", f.domain): alb.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: alb2v1.SchemeGroupVersion.String(),
					Kind:       alb2v1.ALB2Kind,
					Name:       alb.Name,
					UID:        alb.UID,
				},
			},
		},
		Spec: alb2v1.FrontendSpec{
			Port:     f.defaultFt,
			Protocol: m.ProtoHTTP,
		},
	}, metav1.CreateOptions{})
	assert.Nil(ginkgo.GinkgoT(), err, "creating ft")
	Logf("create ft success %s/%s", ft.Namespace, ft.Name)


	albCtl.Init()
	go albCtl.Start(f.ctx)

	f.waitAlbNormal()
}

func (f *Framework) waitAlbNormal() {
	f.WaitNginxConfig("listen.*12345")
	f.WaitPolicy("12345")
}

func (f *Framework) Destroy() {
	f.cancel()
	f.k8sClient.CoreV1().Namespaces().Delete(context.Background(), f.namespace, metav1.DeleteOptions{})
}

func (f *Framework) WaitFile(file string, matcher func(string) bool) {
	err := wait.Poll(Poll, DefaultTimeout, func() (bool, error) {
		fileCtx, err := os.ReadFile(file)
		if err != nil {
			return false, nil
		}
		if matcher(string(fileCtx)) {
			return true, nil
		}
		return false, nil
	})
	assert.Nil(ginkgo.GinkgoT(), err, "wait nginx config contains fail")
}

func regexMatch(text string, matchStr string) bool {
	match, _ := regexp.MatchString(matchStr, text)
	return match
}

func (f *Framework) WaitNginxConfig(regexStr string) {
	f.WaitFile(f.nginxCfgPath, func(raw string) bool {
		match := regexMatch(raw, regexStr)
		Logf("match regex %s in %s %v", regexStr, f.nginxCfgPath, match)
		return match
	})
}

func (f *Framework) WaitPolicy(regexStr string) {
	f.WaitFile(f.policyPath, func(text string) bool {
		match := regexMatch(text, regexStr)
		Logf("match regex %s in %s %v", regexStr, f.policyPath, match)
		return match
	})
}

func log(level string, format string, args ...interface{}) {
	fmt.Fprintf(ginkgo.GinkgoWriter, nowStamp()+": "+level+": "+"envtest framework : "+format+"\n", args...)
}

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

// Logf logs to the INFO logs.
func Logf(format string, args ...interface{}) {
	log("INFO", format, args...)
}
