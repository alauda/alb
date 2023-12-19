package framework

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"

	alog "alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo"

	"k8s.io/client-go/rest"
)

type OperatorEnv struct {
	cfg     *rest.Config
	envtest *EnvtestExt
	Log     logr.Logger
	Ctx     context.Context
	Cancel  context.CancelFunc
	Base    string
	Kt      *Kubectl
	Kc      *K8sClient
	Opext   *AlbOperatorExt
	envcfg  OperatorEnvCfg
	Helm    *Helm
	InitK8s func(ctx context.Context, base string, cfg *rest.Config, l logr.Logger) error
}

type OperatorEnvCfg struct {
	RunOpext      bool
	DefaultValues string
	CsvMode       bool
	mockmetallb   bool
	v4            []string
	v6            []string
	host          []string
}

func (e *OperatorEnvCfg) UseMockLBSvcCtl(v4p, v6p []string) {
	e.mockmetallb = true
	e.v4 = v4p
	e.v6 = v6p
}

func (e *OperatorEnvCfg) UseMockSvcWithHost(v4p, v6p, host []string) {
	e.mockmetallb = true
	e.v4 = v4p
	e.v6 = v6p
	e.host = host
}

func StartOperatorEnvOrDie() *OperatorEnv {
	e := &OperatorEnv{envcfg: OperatorEnvCfg{RunOpext: false}}
	err := e.Start()
	if err != nil {
		panic(err)
	}
	return e
}

func StartOperatorEnvOrDieWithOpt(cfg OperatorEnvCfg, exts ...func(e *OperatorEnv)) *OperatorEnv {
	e := &OperatorEnv{envcfg: cfg}
	for _, ext := range exts {
		ext(e)
	}
	err := e.Start()
	if err != nil {
		panic(err)
	}
	return e
}

func (e *OperatorEnv) Start() error {
	base := InitBase()
	e.Base = base
	l := alog.InitKlogV2(alog.LogCfg{ToFile: base + "/operator.log"})
	e.Log = l
	bootlog := l.WithName(ginkgo.CurrentGinkgoTestDescription().TestText)
	bootlog.Info("base ok", "base", base)
	bootlog.Info("pid", "pid", os.Getpid())
	bootlog.Info("start operator env")
	testEnv := NewEnvtestExt(base, l)
	var err error
	bootlog.Info("start envtest")
	cfg, err := testEnv.Start()
	if err != nil {
		return err
	}
	bootlog.Info("start envtest over")

	ctx, cancel := context.WithCancel(context.Background())
	if e.InitK8s != nil {
		err = e.InitK8s(ctx, base, cfg, l)
		if err != nil {
			cancel()
			return err
		}
	}
	kt := NewKubectl(base, cfg, l)
	kc := NewK8sClient(ctx, cfg)
	// helm install the operator chart
	// get service account token
	opext := NewAlbOperatorExt(ctx, base, cfg, l)
	e.Ctx = ctx
	e.Cancel = cancel
	e.Kt = kt
	e.Kc = kc
	e.Opext = opext
	helm := NewHelm(base, cfg, l)
	e.Helm = helm

	e.cfg = cfg
	e.envtest = testEnv
	err = e.Opext.Init(ctx, e.envcfg.DefaultValues, e.envcfg.CsvMode)
	if err != nil {
		return err
	}
	if e.envcfg.RunOpext {
		go e.Opext.Start(ctx)
	}
	if e.envcfg.mockmetallb {
		go NewMockMetallb(ctx, cfg, e.envcfg.v4, e.envcfg.v6, e.envcfg.host, l).Start()
	}
	return nil
}

func (e *OperatorEnv) Stop() {
	e.Cancel()
	err := e.envtest.Stop()
	if err != nil {
		panic(err)
	}
}

// newLogWriter 创建一个新的 io.Writer，它会立即打印每一行输出。
func newLogWriter(prefix string, log logr.Logger) io.Writer {
	return &logWriter{prefix: prefix, log: log}
}

type logWriter struct {
	prefix string
	log    logr.Logger
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(p))
	for scanner.Scan() {
		txt := scanner.Text()
		// println(txt)
		w.log.Info(txt, "prefix", w.prefix)
	}
	return len(p), scanner.Err()
}
