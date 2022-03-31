package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	albCtl "alauda.io/alb2/alb"
	m "alauda.io/alb2/modules"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albclient "alauda.io/alb2/pkg/client/clientset/versioned"
	"alauda.io/alb2/utils/dirhash"
	"alauda.io/alb2/utils/test_utils"
	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayVersioned "sigs.k8s.io/gateway-api/pkg/client/clientset/gateway/versioned"
)

// a framework represent a alb
type Framework struct {
	k8sClient     kubernetes.Interface
	albClient     albclient.Interface
	gatewayClient gatewayVersioned.Interface
	cfg           *rest.Config
	ctx           context.Context
	cancel        func() // use this function to stop alb
	namespace     string // which ns this alb been deployed
	productNs     string
	AlbName       string
	baseDir       string // dir of alb.log nginx.conf and policy.new
	kubectlDir    string // dir of kubeconfig
	nginxCfgPath  string
	policyPath    string
	albLogPath    string
	domain        string
	defaultFt     int    // this port is meaningless just to make sure alb running healthily
	deployCfg     Config // config used to deploy a alb
}

type Config struct {
	RandomBaseDir bool
	RandomNs      bool
	AlbName       string
	RestCfg       *rest.Config
	InstanceMode  bool
	Project       []string
	Gateway       bool
	PortProbe     bool
}

func InitKubeCfg(cfg *rest.Config) (string, error) {
	kubecfgPath := fmt.Sprintf("%v/.kube/alb-env-test", homedir.HomeDir())
	os.MkdirAll(fmt.Sprintf("%v/.kube", homedir.HomeDir()), os.ModePerm)
	kubecfg, err := KubeConfigFromREST(cfg, "envtest")
	if err != nil {
		return "", err
	}
	err = os.WriteFile(kubecfgPath, kubecfg, os.ModePerm)
	os.Setenv("KUBECONFIG", kubecfgPath)
	os.Setenv("USE_KUBECONFIG", "true")
	Logf("kubecfg %v", kubecfgPath)
	return kubecfgPath, err
}

func CfgFromEnv() *rest.Config {
	kubecfg := os.Getenv("KUBECONFIG")
	cf, err := clientcmd.BuildConfigFromFlags("", kubecfg)
	if err != nil {
		panic(err)
	}
	return cf
}

// TODO use helm install to install crd and cr to k8s
func NewAlb(deployCfg Config) *Framework {
	cfg := deployCfg.RestCfg
	if !(os.Getenv("DEV_MODE") == "true") {
		deployCfg.RandomNs = true
		deployCfg.RandomBaseDir = true
	}
	var baseDir = os.TempDir() + "/alb-e2e-test"
	if deployCfg.RandomBaseDir {
		var err error
		baseDir, err = os.MkdirTemp("", "alb-e2e-test")
		assert.Nil(ginkgo.GinkgoT(), err, "creat temp dir")
	} else {
		os.RemoveAll(baseDir)
		os.MkdirAll(baseDir, os.ModePerm)
	}
	name := deployCfg.AlbName
	if name == "" {
		name = "alb-dev"
	}
	domain := "cpaas.io"
	ns := "cpaas-system"
	if deployCfg.RandomNs {
		ns += "-" + random()
	}
	Logf("alb base dir is %v", baseDir)
	Logf("alb deployed in %s", ns)

	nginxCfgPath := baseDir + "/nginx.conf"
	nginxPolicyPath := baseDir + "/policy.new"

	os.WriteFile(nginxCfgPath, []byte(""), os.ModePerm) // give it a default empty nginx.conf
	Logf("apiserver %s", cfg.Host)

	if deployCfg.PortProbe {
		os.Setenv("ENABLE_PORTPROBE", "true")
	}
	albRoot := getAlbRoot()
	os.Setenv("LB_TYPE", "nginx")
	os.Setenv("ALB_LOCK_TIMEOUT", "5")
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
	kubectlDir := baseDir + "/kubectl"
	os.MkdirAll(kubectlDir, os.ModePerm)
	os.Setenv("ALB_TWEAK_DIRECTORY", twekDir)

	nginxTemplatePath, err := filepath.Abs(filepath.Join(albRoot, "template/nginx/nginx.tmpl"))
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
		baseDir:       baseDir,
		kubectlDir:    kubectlDir,
		cfg:           cfg,
		k8sClient:     k8sClient,
		albClient:     albclient.NewForConfigOrDie(cfg),
		gatewayClient: gatewayVersioned.NewForConfigOrDie(cfg),
		nginxCfgPath:  nginxCfgPath,
		policyPath:    nginxPolicyPath,
		albLogPath:    alblogpath,
		ctx:           ctx,
		cancel:        cancel,
		namespace:     ns,
		AlbName:       name,
		domain:        domain,
		deployCfg:     deployCfg,
		defaultFt:     12345,
	}
}

// GetNamespace get the namespace which alb been deployed
func (f *Framework) GetNamespace() string {
	return f.namespace
}

func getAlbRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Join(path.Dir(filename), "../../../")
	return dir
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
			Name:      f.AlbName,
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
			Name:      fmt.Sprintf("%s-%05d", f.AlbName, f.defaultFt),
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

	if f.deployCfg.Gateway {
		f.gatewayClient.GatewayV1alpha2().GatewayClasses().Create(f.ctx, &gatewayType.GatewayClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: f.AlbName,
			},
			Spec: gatewayType.GatewayClassSpec{
				ControllerName: gatewayType.GatewayController(fmt.Sprintf("alb2.gateway.%v/%v", f.domain, f.AlbName)),
			},
		}, metav1.CreateOptions{})
		os.Setenv("ENABLE_GATEWAY", "true")
	}
	err = f.InitSvc(f.namespace, f.AlbName, []string{"172.168.1.1"})
	if err != nil {
		Logf("err %v", err)
	}

	albCtl.Init()
	go albCtl.Start(f.ctx)

	f.waitAlbNormal()
}

func (f *Framework) GetAlbPodIp() []string {
	return []string{"172.168.1.1"}
}

func (f *Framework) waitAlbNormal() {
	f.WaitNginxConfigStr("listen.*12345")
	f.WaitPolicyRegex("12345")
}

func (f *Framework) Destroy() {
	f.cancel()
}

func (f *Framework) WaitFile(file string, matcher func(string) (bool, error)) {
	err := wait.Poll(Poll, DefaultTimeout, func() (bool, error) {
		fileCtx, err := os.ReadFile(file)
		if err != nil {
			return false, nil
		}
		ok, err := matcher(string(fileCtx))
		if err != nil {
			return false, err
		}
		return ok, nil
	})
	assert.Nil(ginkgo.GinkgoT(), err, "wait nginx config contains fail")
}

func regexMatch(text string, matchStr string) bool {
	match, _ := regexp.MatchString(matchStr, text)
	return match
}

func (f *Framework) WaitNginxConfig(check func(raw string) (bool, error)) {
	f.WaitFile(f.nginxCfgPath, check)
}

func (f *Framework) WaitNginxConfigStr(regexStr string) {
	f.WaitFile(f.nginxCfgPath, func(raw string) (bool, error) {
		match := regexMatch(raw, regexStr)
		Logf("match regex %s in %s %v", regexStr, f.nginxCfgPath, match)
		return match, nil
	})
}

func (f *Framework) WaitPolicyRegex(regexStr string) {
	f.WaitFile(f.policyPath, func(raw string) (bool, error) {
		match := regexMatch(raw, regexStr)
		Logf("match regex %s in %s %v", regexStr, f.policyPath, match)
		return match, nil
	})
}

func (f *Framework) WaitNgxPolicy(fn func(p NgxPolicy) (bool, error)) {
	f.WaitFile(f.policyPath, func(raw string) (bool, error) {
		Logf("p  %s", raw)
		p := NgxPolicy{}
		err := json.Unmarshal([]byte(raw), &p)
		if err != nil {
			return false, fmt.Errorf("wait nginx policy fial err %v raw -- %s --", err, raw)
		}
		return fn(p)
	})
}

func (f *Framework) WaitPolicy(fn func(raw string) bool) {
	f.WaitFile(f.policyPath, func(raw string) (bool, error) {
		match := fn(raw)
		Logf("match in %s %v", f.policyPath, match)
		return match, nil
	})
}

func (f *Framework) WaitIngressRule(ingresName, ingressNs string, size int) []alb2v1.Rule {
	rulesChan := make(chan []alb2v1.Rule, 1)
	err := wait.Poll(Poll, DefaultTimeout, func() (bool, error) {

		selType := fmt.Sprintf("alb2.%s/source-type=ingress", f.domain)
		selName := fmt.Sprintf("alb2.%s/source-name-hash=%s", f.domain, dirhash.LabelSafetHash(fmt.Sprintf("%s.%s", ingresName, ingressNs)))
		sel := selType + "," + selName
		rules, err := f.albClient.CrdV1().Rules(f.namespace).List(f.ctx, metav1.ListOptions{LabelSelector: sel})
		if err != nil {
			Logf("get rule for ingress %s/%s sel -%s- fail %s", ingressNs, ingresName, sel, err)
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

func (f *Framework) Wait(fn func() (bool, error)) {
	err := wait.Poll(Poll, DefaultTimeout, fn)
	assert.Nil(ginkgo.GinkgoT(), err, "wait fail")
}

func (f *Framework) InitProductNs(nsprefix string, project string) {
	ns := nsprefix
	if f.deployCfg.RandomNs {
		ns += "-" + random()
	}
	f.productNs = ns
	Logf("init product ns %s", f.productNs)
	f.k8sClient.CoreV1().Namespaces().Create(
		f.ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: f.productNs,
				Labels: map[string]string{
					fmt.Sprintf("%s/project", f.domain): project,
				},
			},
		},
		metav1.CreateOptions{},
	)
}

func (f *Framework) GetProductNs() string {
	return f.productNs
}

func (f *Framework) GetK8sClient() kubernetes.Interface {
	return f.k8sClient
}

func (f *Framework) GetAlbClient() albclient.Interface {
	return f.albClient
}

func (f *Framework) GetGatewayClient() gatewayVersioned.Interface {
	return f.gatewayClient
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
	SvcPort   map[string]struct { // key svc.port.name which match ep.port.name
		Protocol   corev1.Protocol
		Port       int32
		Target     intstr.IntOrString
		TargetPort int32
		TargetName string // the name match pod.port.name
	}
	Eps     []string
	Ingress struct {
		Name string
		Host string
		Path string
		Port intstr.IntOrString
	}
}

func (f *Framework) InitIngressCase(ingressCase IngressCase) {
	var svcPort []corev1.ServicePort
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
	svc, err := f.GetK8sClient().CoreV1().Services(ingressCase.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
	Logf("svc port %+v", svcPort)
	Logf("created svc %+v", svc)
	assert.Nil(ginkgo.GinkgoT(), err, "")
	subSetAddress := []corev1.EndpointAddress{}
	for _, address := range ingressCase.Eps {
		subSetAddress = append(subSetAddress, corev1.EndpointAddress{
			IP: address,
		})
	}
	subSetPort := []corev1.EndpointPort{}
	for svcPortName, p := range ingressCase.SvcPort {
		subSetPort = append(subSetPort,
			corev1.EndpointPort{
				Port:     p.TargetPort,
				Protocol: corev1.ProtocolTCP,
				Name:     svcPortName,
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
}

func (f *Framework) InitSvc(ns, name string, ep []string) error {
	Logf("init svc %v %v %v", ns, name, ep)
	_, err := f.k8sClient.CoreV1().Services(ns).Create(f.ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 80},
			}},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	address := []corev1.EndpointAddress{}
	for _, ip := range ep {
		address = append(address, corev1.EndpointAddress{IP: ip})
	}
	_, err = f.k8sClient.CoreV1().Endpoints(ns).Create(f.ctx, &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels:    map[string]string{"kube-app": name},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: address,
				Ports:     []corev1.EndpointPort{{Port: 80}},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (f *Framework) InitDefaultSvc(name string, ep []string) {
	ns := f.productNs
	f.InitSvc(ns, name, ep)
}

func (f *Framework) CreateIngress(name string, path string, svc string, port int) {
	ns := f.productNs
	_, err := f.GetK8sClient().NetworkingV1().Ingresses(ns).Create(context.Background(), &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     path,
									PathType: (*networkingv1.PathType)(ToPointOfString(string(networkingv1.PathTypeImplementationSpecific))),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: svc,
											Port: networkingv1.ServiceBackendPort{
												Number: int32(port),
											},
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
}

func (f *Framework) CreateFt(port int, protocol string, svcName string, svcNs string) {
	name := fmt.Sprintf("%s-%05d", f.AlbName, port)
	if protocol == "udp" {
		name = name + "-udp"
	}
	ft := alb2v1.Frontend{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.namespace,
			Name:      name,
			Labels: map[string]string{
				"alb2.cpaas.io/name": f.AlbName,
			},
		},
		Spec: alb2v1.FrontendSpec{
			Port:     port,
			Protocol: alb2v1.FtProtocol(protocol),
			ServiceGroup: &alb2v1.ServiceGroup{Services: []alb2v1.Service{
				{
					Name:      svcName,
					Namespace: svcNs,
					Port:      80,
				},
			}},
		},
	}
	f.albClient.CrdV1().Frontends(f.namespace).Create(context.Background(), &ft, metav1.CreateOptions{})
}

func (f *Framework) WaitFtState(name string, check func(ft *alb2v1.Frontend) (bool, error)) *alb2v1.Frontend {
	var ft *alb2v1.Frontend
	var err error
	err = wait.Poll(Poll, DefaultTimeout, func() (bool, error) {
		ft, err = f.albClient.CrdV1().Frontends(f.GetNamespace()).Get(context.Background(), name, metav1.GetOptions{})
		Logf("try get ft %s/%s ft %v", f.GetNamespace(), name, err)
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		ok, err := check(ft)
		if err == nil {
			return ok, nil
		}
		return ok, err
	})
	assert.NoError(ginkgo.GinkgoT(), err)
	return ft
}

func (f *Framework) WaitFt(name string) *alb2v1.Frontend {
	return f.WaitFtState(name, func(ft *alb2v1.Frontend) (bool, error) {
		if ft != nil {
			return true, nil
		}
		return false, nil
	})
}

func (f *Framework) WaitAlbState(name string, check func(alb *alb2v1.ALB2) (bool, error)) *alb2v1.ALB2 {
	var globalAlb *alb2v1.ALB2
	err := wait.Poll(Poll, DefaultTimeout, func() (bool, error) {
		alb, err := f.albClient.CrdV1().ALB2s(f.GetNamespace()).Get(context.Background(), name, metav1.GetOptions{})
		Logf("try get alb %s/%s alb %v", f.GetNamespace(), name, err)
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		ok, err := check(alb)
		if err == nil {
			globalAlb = alb
			return ok, nil
		}
		return ok, err
	})
	assert.NoError(ginkgo.GinkgoT(), err)
	return globalAlb
}

func RandomFile(base string, file string) (string, error) {
	p := path.Join(base, random())
	err := os.WriteFile(p, []byte(file), os.ModePerm)
	return p, err
}

func (f *Framework) KubectlApply(yaml string, options ...string) (string, error) {
	p, err := RandomFile(f.kubectlDir, yaml)
	if err != nil {
		return "", err
	}
	cmds := []string{"apply", "-f", p, "--kubeconfig", os.Getenv("KUBECONFIG")}
	cmds = append(cmds, options...)
	Logf("cmds: kubectl %v", strings.Join(cmds, " "))
	return Kubectl(cmds...)
}

func (f *Framework) CreateTlsSecret(domain, name, ns string) (*corev1.Secret, error) {
	key, crt, err := test_utils.GenCert(domain)
	if err != nil {
		return nil, err
	}
	secret, err := f.k8sClient.CoreV1().Secrets(ns).Create(f.ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string][]byte{
			"tls.key": []byte(key),
			"tls.crt": []byte(crt),
		},
		Type: corev1.SecretTypeTLS,
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return secret, nil
}
