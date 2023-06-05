package framework

// NOTE framework 这个package和test_utils这个package的区别在于在framework中，为了模拟环境我们要引用一些业务的代码，而test_utils是纯的，无依赖的.
// TODO 有些ext需要迁移到test_utils中。
// TODO 现在edge环境流水线已经可以用kind了，要支持kind，这样可以测一些更复杂的场景，比如健康检查。
// TODO framework 这个名字有点奇怪，test-framwork 或者test-helper？
import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"alauda.io/alb2/config"
	albCtl "alauda.io/alb2/controller/alb"
	m "alauda.io/alb2/modules"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albclient "alauda.io/alb2/pkg/client/clientset/versioned"
	tu "alauda.io/alb2/utils/test_utils"
	"github.com/onsi/ginkgo"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"

	alblog "alauda.io/alb2/utils/log"
	gatewayVersioned "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

type AlbCtl struct {
	kind string
}

type AlbInfo struct {
	AlbName      string
	AlbNs        string
	Domain       string
	NginxCfgPath string
	PolicyPath   string
}

// a test framework that simulate a test env for alb.
type Framework struct {
	cfg          *rest.Config
	fCtx         context.Context
	fCancel      func() // use this function to stop test framework
	albCtx       context.Context
	albCancel    func() // use this function to stop alb
	ctlChan      chan AlbCtl
	namespace    string // which ns this alb been deployed
	productNsMap map[string]string
	productNs    string
	AlbName      string
	baseDir      string // dir of alb.log nginx.conf and policy.new
	nginxCfgPath string
	policyPath   string
	albLogPath   string
	domain       string
	defaultFt    alb2v1.PortNumber // this port is meaningless just to make sure alb running healthily
	deployCfg    Config            // config used to deploy an alb
	*tu.Kubectl
	*tu.K8sClient
	*AlbWaitFileExt
	*AlbHelper
	*tu.Helm
	*AlbOperatorExt
	*GatewayAssert
}

// TODO: 除了测试用的配置之外，其他的应该直接用alb的cr
type Config struct {
	Mode          config.Mode
	NetworkMode   config.ControllerNetWorkMode
	RandomBaseDir bool
	RandomNs      bool
	AlbName       string
	AlbAddress    string // address set in alb cr.
	RestCfg       *rest.Config
	InstanceMode  bool
	Project       []string
	Gateway       bool
	GatewayMode   config.GatewayMode
	GatewayName   string
	PodName       string
	PortProbe     bool
	OverrideEnv   map[string]string
}

var DefaultGatewayClass = Config{
	Mode:        config.Controller,
	NetworkMode: config.Host,
	Gateway:     true,
	GatewayMode: config.GatewayClass,
}

// 将cfg转换为kubectl 配置文件的格式设置 KUBECONFIG的环境变量
func InitKubeCfgEnv(cfg *rest.Config) (string, error) {
	kubecfgPath := ""
	if os.Getenv("DEV_MODE") == "true" {
		os.MkdirAll(fmt.Sprintf("%v/.kube", homedir.HomeDir()), os.ModePerm)
		kubecfgPath = fmt.Sprintf("%v/.kube/alb-env-test", homedir.HomeDir())

	} else {
		baseDir, err := os.MkdirTemp("", "alb-e2e-test")
		if err != nil {
			return "", err
		}
		kubecfgPath = path.Join(baseDir, "alb-env-test")
	}

	kubecfg, err := tu.KubeConfigFromREST(cfg, "envtest")
	if err != nil {
		return "", err
	}
	err = os.WriteFile(kubecfgPath, kubecfg, os.ModePerm)
	os.Setenv("KUBECONFIG", kubecfgPath)
	os.Setenv("USE_KUBECONFIG", "true")
	Logf("kubecfg %v", kubecfgPath)
	return kubecfgPath, err
}

type ReadAlbFile interface {
	ReadFile(p string) (string, error)
}

// init what we need before deploy alb
func NewAlb(deployCfg Config) *Framework {
	cfg := deployCfg.RestCfg

	if !(os.Getenv("DEV_MODE") == "true") {
		deployCfg.RandomBaseDir = true
	}

	if deployCfg.AlbAddress == "" {
		deployCfg.AlbAddress = "127.0.0.1"
	}

	if deployCfg.PodName == "" {
		deployCfg.PodName = "p1"
	}
	if deployCfg.NetworkMode == "" {
		deployCfg.NetworkMode = "host"
	}
	if deployCfg.OverrideEnv == nil {
		deployCfg.OverrideEnv = map[string]string{}
	}

	// use random base dir, since than we could run mutli alb e2e at once
	var baseDir = tu.InitBase()

	name := deployCfg.AlbName
	if name == "" {
		name = "alb-dev"
	}
	domain := "cpaas.io"
	ns := "cpaas-system"
	Logf("alb base dir is %v", baseDir)
	Logf("alb deployed in %s", ns)

	nginxCfgPath := baseDir + "/nginx.conf"
	nginxPolicyPath := baseDir + "/policy.new"

	os.WriteFile(nginxCfgPath, []byte(""), os.ModePerm) // give it a default empty nginx.conf
	Logf("apiserver %s", cfg.Host)

	albRoot := GetAlbRoot()

	twekDir := baseDir + "/tweak"
	os.MkdirAll(twekDir, os.ModePerm)

	nginxTemplatePath, err := filepath.Abs(filepath.Join(albRoot, "template/nginx/nginx.tmpl"))
	assert.Nil(ginkgo.GinkgoT(), err, "nginx template")
	assert.FileExists(ginkgo.GinkgoT(), nginxTemplatePath, "nginx template")

	statusDir := baseDir + "/last_status"
	os.MkdirAll(statusDir, os.ModePerm)

	alblogpath := baseDir + "/alb.log"
	fctx, fcancel := context.WithCancel(context.Background())
	client := tu.NewK8sClient(fctx, cfg)
	k := tu.NewKubectl(baseDir, deployCfg.RestCfg, tu.GinkgoLog())

	albctx, albcancel := context.WithCancel(fctx)

	f := &Framework{
		baseDir:      baseDir,
		cfg:          cfg,
		nginxCfgPath: nginxCfgPath,
		policyPath:   nginxPolicyPath,
		albLogPath:   alblogpath,
		fCtx:         fctx,
		fCancel:      fcancel,
		albCtx:       albctx,
		albCancel:    albcancel,
		AlbName:      name,
		namespace:    ns,
		domain:       domain,
		deployCfg:    deployCfg,
		defaultFt:    12345,
		Kubectl:      k,
		K8sClient:    client,
		ctlChan:      make(chan AlbCtl, 10),
	}
	f.AlbWaitFileExt = NewAlbWaitFileExt(f, f.GetAlbInfo())
	f.Helm = tu.NewHelm(baseDir, deployCfg.RestCfg, tu.GinkgoLog())
	f.AlbOperatorExt = NewAlbOperatorExt(fctx, baseDir, deployCfg.RestCfg)
	f.GatewayAssert = NewGatewayAssert(client, fctx)
	f.AlbHelper = &AlbHelper{
		K8sClient: client,
		AlbInfo:   f.GetAlbInfo(),
	}

	Logf("init ns %v", f.namespace)
	tu.GinkgoAssert(f.CreateNsIfNotExist(f.namespace), "")
	env := map[string]string{
		"KUBERNETES_SERVER":            cfg.Host,
		"KUBERNETES_BEARERTOKEN":       cfg.BearerToken,
		"NGINX_TEMPLATE_PATH":          nginxTemplatePath,
		"NEW_CONFIG_PATH":              nginxCfgPath + ".new",
		"OLD_CONFIG_PATH":              nginxCfgPath,
		"NEW_POLICY_PATH":              nginxPolicyPath,
		"ALB_E2E_TEST_CONTROLLER_ONLY": "true",
		"ALB_TWEAK_DIRECTORY":          twekDir,
		"VIPER_BASE":                   albRoot,
		"ALB_STATUSFILE_PARENTPATH":    statusDir,
		"ALB_LOG_EXT":                  "true",
		"ALB_LOG_FILE":                 alblogpath,
		"ALB_LOG_LEVEL":                "3",
		"ALB_DISABLE_LOG_STDERR":       "true",
		"ALB_LEADER_LEASE_DURATION":    "3000",
		"ALB_LEADER_RENEW_DEADLINE":    "2000",
		"ALB_LEADER_RETRY_PERIOD":      "1000",
		"MY_POD_NAME":                  f.deployCfg.PodName,
	}
	for k, v := range env {
		deployCfg.OverrideEnv[k] = v
	}
	return f
}

func (f *Framework) GetAlbInfo() AlbInfo {
	return AlbInfo{
		AlbName:      f.AlbName,
		AlbNs:        f.namespace,
		Domain:       f.domain,
		NginxCfgPath: f.nginxCfgPath,
		PolicyPath:   f.policyPath,
	}
}

func (f *Framework) ReadFile(p string) (string, error) {
	ret, err := os.ReadFile(p)
	return string(ret), err
}

// GetNamespace get the namespace which alb been deployed
func (f *Framework) GetNamespace() string {
	return f.namespace
}

func (f *Framework) GetCtx() context.Context {
	return f.fCtx
}

func GetAlbRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Join(path.Dir(filename), "../../../")
	return dir
}

func (f *Framework) toAlbCfg() string {
	if f.deployCfg.Gateway && f.deployCfg.NetworkMode == "container" {
		return tu.Template(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: {{ .name }} 
    namespace: {{ .ns }}
    labels:
        alb.cpaas.io/managed-by: alb-operator
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        mode: controller
        networkMode: {{.networkMode}}
        loadbalancerName: {{.name}}
        projects: {{.projects}}
        albEnable: true
        enableIngress: "false"
        gateway:
            enable: true
            mode: gateway
            gatewayModeCfg:
                name: "{{.gatewayName}}"
        replicas: 3
        `, map[string]interface{}{
			"name":        f.AlbName,
			"ns":          f.AlbNs,
			"gatewayName": f.deployCfg.GatewayName,
			"networkMode": f.deployCfg.NetworkMode,
			"projects":    f.deployCfg.Project,
		})
	}
	return tu.Template(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: {{ .name }} 
    namespace: {{ .ns }}
    labels:
        alb.cpaas.io/managed-by: alb-operator
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        mode: controller
        address: {{.address}}
        enablePortProject: {{.portMode}}
        networkMode: {{.networkMode}}
        loadbalancerName: {{.name}}
        projects:    {{.projects}}
        albEnable: true
        enableIngress: "true"
        gateway:
            enable: {{ .gateway }}
            mode: gatewayclass
        replicas: 3
        `, map[string]interface{}{
		"portMode":    !f.deployCfg.InstanceMode,
		"networkMode": f.deployCfg.NetworkMode,
		"gateway":     f.deployCfg.Gateway,
		"name":        f.AlbName,
		"ns":          f.AlbNs,
		"projects":    f.deployCfg.Project,
		"address":     f.deployCfg.AlbAddress,
	})
}

// TODO alb-operator should provide a cli interface to deploy a alb directly.
func (f *Framework) Init() {
	// TODO we should generate deploy config directly..
	cfg := f.toAlbCfg()
	Logf("alb cfg %v", cfg)
	f.AssertDeploy(types.NamespacedName{Namespace: f.namespace, Name: f.AlbName}, cfg, nil)
	env, _ := f.GetDeploymentEnv(f.namespace, f.AlbName, "alb2")
	for key, val := range env {
		Logf("env %v %v", key, val)
		os.Setenv(key, val)
	}

	for key, val := range f.deployCfg.OverrideEnv {
		Logf("override env %v %v", key, val)
		os.Setenv(key, val)
	}
	go f.StartTestAlbLoop()
	// TODO a better way
	if os.Getenv("ALB_RELOAD_NGINX") == "false" {
		return
	}
	f.initDefaultFt()
	f.waitAlbNormal()
}

func (f *Framework) waitAlbNormal() {
	f.WaitNginxConfigStr("listen.*12345")
	f.WaitPolicyRegex("12345")
}

func (f *Framework) initDefaultFt() {
	_, err := f.GetAlbClient().CrdV1().Frontends(f.namespace).Create(f.fCtx, &alb2v1.Frontend{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.namespace,
			Name:      fmt.Sprintf("%s-%05d", f.AlbName, f.defaultFt),
			// the most import part
			Labels: map[string]string{
				fmt.Sprintf("alb2.%s/name", f.domain): f.AlbName,
			},
		},
		Spec: alb2v1.FrontendSpec{
			Port:     alb2v1.PortNumber(f.defaultFt),
			Protocol: m.ProtoHTTP,
		},
	}, metav1.CreateOptions{})

	tu.GinkgoAssert(err, "init defualt ft fail")
}

func (f *Framework) StartTestAlbLoop() {
	for {
		err := config.Init()
		if err != nil {
			panic(err)
		}
		cfg := config.GetConfig()
		alb := albCtl.NewAlb(f.albCtx, f.cfg, cfg, alblog.L())
		alb.Start()
		ctl := <-f.ctlChan
		if ctl.kind == "stop" {
			break
		}
		if ctl.kind == "restart" {
			continue
		}
		panic(fmt.Sprintf("unknow event %v", ctl))
	}
}

func (f *Framework) RestartAlb() {
	f.ctlChan <- AlbCtl{
		kind: "restart",
	}
	oldc := f.albCancel
	ctx, cancel := context.WithCancel(f.fCtx)
	f.albCtx = ctx
	f.albCancel = cancel
	oldc()
}

func (f *Framework) Destroy() {
	f.ctlChan <- AlbCtl{
		kind: "stop",
	}
	f.albCancel()
	time.Sleep(time.Second * 3)
	f.fCancel()
}

func (f *Framework) Wait(fn func() (bool, error)) {
	err := wait.Poll(Poll, DefaultTimeout, fn)
	assert.Nil(ginkgo.GinkgoT(), err, "wait fail")
}

// func WaitT[T any](fn func() (T, error)) {
// 	err := wait.Poll(Poll, DefaultTimeout, func() (bool, error) {

// 	})
// 	assert.Nil(ginkgo.GinkgoT(), err, "wait fail")
// }

func (f *Framework) InitProductNs(nsprefix string, project string) string {
	ns := f.InitProductNsWithOpt(ProductNsOpt{
		Prefix:  nsprefix,
		Project: project,
	})
	f.productNs = ns
	return ns
}

type ProductNsOpt struct {
	Prefix  string
	Ns      string
	Project string
	Labels  map[string]string
}

func (f *Framework) InitProductNsWithOpt(opt ProductNsOpt) string {
	if opt.Labels == nil {
		opt.Labels = map[string]string{}
	}
	opt.Labels[fmt.Sprintf("%s/project", f.domain)] = opt.Project
	opt.Ns = opt.Prefix
	if f.deployCfg.RandomNs {
		opt.Ns = opt.Prefix + "-" + random()
	}

	ns, err := f.GetK8sClient().CoreV1().Namespaces().Create(
		f.fCtx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   opt.Ns,
				Labels: opt.Labels,
			},
		},
		metav1.CreateOptions{},
	)
	assert.Nil(ginkgo.GinkgoT(), err, "create ns fail")
	return ns.Name
}

func (f *Framework) GetProductNs() string {
	return f.productNs
}

func (f *Framework) GetK8sClient() kubernetes.Interface {
	return f.K8sClient.GetK8sClient()
}

func (f *Framework) GetAlbClient() albclient.Interface {
	return f.K8sClient.GetAlbClient()
}

func (f *Framework) GetGatewayClient() gatewayVersioned.Interface {
	return f.K8sClient.GetGatewayClient()
}

func (f *Framework) InitDefaultSvc(name string, ep []string) {
	opt := SvcOpt{
		Ns:   f.productNs,
		Name: name,
		Ep:   ep,
		Ports: []corev1.ServicePort{
			{
				Port: 80,
			},
		},
	}
	if strings.Contains(name, "udp") {
		opt.Ports[0].Protocol = "UDP"
	}
	f.initSvcWithOpt(opt)
}

func (f *Framework) GetAlbAddress() string {
	return f.deployCfg.AlbAddress
}

func NewContainerModeCfg(gateway string, host bool, cfg *rest.Config, name string) Config {
	network := config.Container
	if host {
		network = config.Host
	}
	return Config{
		Mode:        "controller",
		Gateway:     true,
		GatewayMode: "gateway",
		GatewayName: gateway,
		AlbName:     name,
		NetworkMode: network,
		RestCfg:     cfg,
	}
}

func (f *Framework) CreateTlsSecret(domain, name, ns string) (*corev1.Secret, error) {
	key, crt, err := tu.GenCert(domain)
	if err != nil {
		return nil, err
	}
	secret, err := f.GetK8sClient().CoreV1().Secrets(ns).Create(f.fCtx, &corev1.Secret{
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

type SvcOptPort struct {
	port        int
	Protocol    string
	AppProtocol *string
}

type SvcOpt struct {
	Ns    string
	Name  string
	Ep    []string
	Ports []corev1.ServicePort
}

func (f *Framework) initSvcWithOpt(opt SvcOpt) error {
	ns := opt.Ns
	name := opt.Name
	ep := opt.Ep

	service_spec := corev1.ServiceSpec{
		Ports: opt.Ports,
	}
	epPorts := lo.Map(opt.Ports, func(p corev1.ServicePort, _ int) corev1.EndpointPort {
		return corev1.EndpointPort{
			Name:        p.Name,
			Protocol:    p.Protocol,
			Port:        p.Port,
			AppProtocol: p.AppProtocol,
		}
	})
	_, err := f.GetK8sClient().CoreV1().Services(ns).Create(f.albCtx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: service_spec,
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	address := []corev1.EndpointAddress{}
	for _, ip := range ep {
		address = append(address, corev1.EndpointAddress{IP: ip})
	}
	_, err = f.GetK8sClient().CoreV1().Endpoints(ns).Create(f.fCtx, &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels:    map[string]string{"kube-app": name},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: address,
				Ports:     epPorts,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (f *Framework) InitSvcWithOpt(opt SvcOpt) {
	err := f.initSvcWithOpt(opt)
	assert.NoError(ginkgo.GinkgoT(), err)
}

func (f *Framework) InitSvc(ns, name string, ep []string) {
	opt := SvcOpt{
		Ns:   ns,
		Name: name,
		Ep:   ep,
		Ports: []corev1.ServicePort{
			{
				Port: 80,
			},
		},
	}
	f.initSvcWithOpt(opt)
}
