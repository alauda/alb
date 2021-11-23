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
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	cancel       func() // use this function to stop alb
	namespace    string // which ns this alb been deployed
	albName      string
	baseDir      string // dir of alb.log nginx.conf and policy.new
	nginxCfgPath string
	policyPath   string
	albLogPath   string
	domain       string
	defaultFt    int    // this port is meaningless just to make sure alb running healthily
	deployCfg    Config // config used to deploy a alb
}

type Config struct {
	RandomBaseDir bool
	RestCfg       *rest.Config
	InstanceMode  bool
	Project       []string
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

func NewAlb(deployCfg Config) *Framework {
	cfg := deployCfg.RestCfg

	var baseDir = os.TempDir() + "/alb-e2e-test"
	if deployCfg.RandomBaseDir {
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
	os.Setenv("INTERVAL", "1")
	os.Setenv("ALB_RELOAD_TIMEOUT", "5")

	statusDir := baseDir + "/last_status"
	os.Setenv("ALB_STATUSFILE_PARENTPATH", statusDir)
	os.MkdirAll(statusDir, os.ModePerm)

	os.Setenv("ALB_LOG_EXT", "true")
	alblogpath := baseDir + "/alb.log"
	os.Setenv("ALB_LOG_FILE", alblogpath)
	os.Setenv("ALB_DISABLE_LOG_STDERR", "true")

	// enable ingress
	os.Setenv("ALB_SERVE_INGRESS", "true")

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
		deployCfg:    deployCfg,
		defaultFt:    12345,
	}
}

// get the namespace which alb been deployed
func (f *Framework) GetNamespace() string {
	return f.namespace
}

func (f *Framework) Init() {
	_, err := CreateKubeNamespace(f.namespace, f.k8sClient)
	assert.Nil(ginkgo.GinkgoT(), err, "creating ns")

	// create alb
	labelsInAlb := map[string]string{}
	if f.deployCfg.InstanceMode {
		labelsInAlb[fmt.Sprintf("%s/role", f.domain)] = "instance"
		for _, p := range f.deployCfg.Project {
			labelsInAlb[fmt.Sprintf("project.%s/%s", f.domain, p)] = "true"
		}
	} else {
		labelsInAlb[fmt.Sprintf("%s/role", f.domain)] = "port"
	}
	Logf("label in alb is %+v", labelsInAlb)
	alb, err := f.albClient.CrdV1().ALB2s(f.namespace).Create(f.ctx, &alb2v1.ALB2{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.namespace,
			Name:      f.albName,
			Labels:    labelsInAlb,
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
	f.WaitPolicyRegex("12345")
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

func (f *Framework) WaitPolicyRegex(regexStr string) {
	f.WaitFile(f.policyPath, func(raw string) bool {
		match := regexMatch(raw, regexStr)
		Logf("match regex %s in %s %v", regexStr, f.policyPath, match)
		return match
	})
}

func (f *Framework) WaitPolicy(fn func(raw string) bool) {
	f.WaitFile(f.policyPath, func(raw string) bool {
		match := fn(raw)
		Logf("match in %s %v", f.policyPath, match)
		return match
	})
}

func (f *Framework) WaitIngressRule(ingresName, ingressNs string, size int) []alb2v1.Rule {
	rulesChan := make(chan []alb2v1.Rule, 1)
	err := wait.Poll(Poll, DefaultTimeout, func() (bool, error) {

		selType := fmt.Sprintf("alb2.%s/source-type=ingress", f.domain)
		selName := fmt.Sprintf("alb2.%s/source-name=%s.%s", f.domain, ingresName, ingressNs)
		sel := selType + "," + selName
		rules, err := f.albClient.CrdV1().Rules(f.namespace).List(f.ctx, metav1.ListOptions{LabelSelector: sel})
		if err != nil {
			Logf("get rule in %s sel %s fail %s", err)
		}
		if len(rules.Items) == size {
			rulesChan <- rules.Items
			return true, nil
		}
		return false, nil
	})
	assert.Nil(ginkgo.GinkgoT(), err, "wait rule fail")
	rules := <-rulesChan
	return rules
}

func (f *Framework) EnsureNs(namespace string, project string) error {
	f.k8sClient.CoreV1().Namespaces().Create(
		f.ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Labels: map[string]string{
					fmt.Sprintf("%s/project", f.domain): project,
				},
			},
		},
		metav1.CreateOptions{},
	)
	return nil
}

func (f *Framework) GetK8sClient() kubernetes.Interface {
	return f.k8sClient
}

func (f *Framework) DestroyNs(s string) {
	f.k8sClient.CoreV1().Namespaces().Delete(context.Background(), s, metav1.DeleteOptions{})
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

// ingress service and end point
type IngressCase struct {
	Namespace string
	Name      string
	SvcPort   map[string]struct {
		Protocol   corev1.Protocol
		Port       int32
		Target     intstr.IntOrString
		TargetPort int32
		TargetName string
	}
	Eps     []string
	Ingress struct {
		Name string
		Host string
		Path string
		Port intstr.IntOrString
	}
}

func (f *Framework) InitIngressCase(ingressCase IngressCase) error {
	svcPort := []corev1.ServicePort{}
	for name, p := range ingressCase.SvcPort {
		svcPort = append(svcPort,
			corev1.ServicePort{
				Port:       p.Port,
				Protocol:   corev1.ProtocolTCP,
				Name:       name,
				TargetPort: p.Target,
			},
		)
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressCase.Name,
			Namespace: ingressCase.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports:    svcPort,
			Selector: map[string]string{"kube-app": ingressCase.Name},
		},
	}
	_, err := f.GetK8sClient().CoreV1().Services(ingressCase.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
	assert.Nil(ginkgo.GinkgoT(), err, "")
	subSetAddress := []corev1.EndpointAddress{}
	for _, addres := range ingressCase.Eps {
		subSetAddress = append(subSetAddress, corev1.EndpointAddress{
			IP: addres,
		})
	}
	subSetPort := []corev1.EndpointPort{}
	for _, p := range ingressCase.SvcPort {
		subSetPort = append(subSetPort,
			corev1.EndpointPort{
				Port:     p.TargetPort,
				Protocol: corev1.ProtocolTCP,
				Name:     p.TargetName,
			},
		)
	}
	subSet := corev1.EndpointSubset{
		NotReadyAddresses: []corev1.EndpointAddress{},
		Addresses:         subSetAddress,
		Ports:             subSetPort,
	}

	ep := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ingressCase.Namespace,
			Name:      ingressCase.Name,
			Labels:    map[string]string{"kube-app": ingressCase.Name},
		},
		Subsets: []corev1.EndpointSubset{subSet}}

	_, err = f.GetK8sClient().CoreV1().Endpoints(ingressCase.Namespace).Create(context.Background(), ep, metav1.CreateOptions{})
	assert.Nil(ginkgo.GinkgoT(), err, "")
	ingressPort := networkingv1.ServiceBackendPort{}
	if ingressCase.Ingress.Port.IntVal != 0 {
		ingressPort.Number = ingressCase.Ingress.Port.IntVal
	} else {
		ingressPort.Name = ingressCase.Ingress.Port.StrVal
	}

	_, err = f.GetK8sClient().NetworkingV1().Ingresses(ingressCase.Namespace).Create(context.Background(), &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ingressCase.Namespace,
			Name:      ingressCase.Ingress.Name,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: ingressCase.Ingress.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     ingressCase.Ingress.Path,
									PathType: (*networkingv1.PathType)(ToPointOfString(string(networkingv1.PathTypeImplementationSpecific))),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: ingressCase.Name,
											Port: ingressPort,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})

	assert.Nil(ginkgo.GinkgoT(), err, "")
	return nil
}
