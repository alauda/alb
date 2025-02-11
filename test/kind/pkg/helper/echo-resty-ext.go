package helper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"github.com/xorcare/pointer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"

	_ "embed"

	. "alauda.io/alb2/utils/test_utils"
)

//go:embed echo-resty.yaml
var EchoRestyTemplate string

type Echo struct {
	log  logr.Logger
	base string
	k    *Kubectl
	kc   *K8sClient
	cfg  EchoCfg
}

func NewEchoResty(base string, cfg *rest.Config, log logr.Logger) *Echo {
	return &Echo{
		log:  log,
		base: base,
		k:    NewKubectl(base, cfg, log),
		kc:   NewK8sClient(context.Background(), cfg),
	}
}

type EchoCfg struct {
	Image          string
	Ns             string
	Name           string
	Ip             string
	Lb             string
	PodPort        string
	PodHostPort    string
	Raw            string
	DefaultIngress *bool
}

func (e *Echo) Deploy(cfg EchoCfg) (*Echo, error) {
	if cfg.Ip == "" {
		cfg.Ip = "v4"
	}
	if cfg.DefaultIngress == nil {
		cfg.DefaultIngress = pointer.Bool(true)
	}
	if cfg.PodPort == "" {
		cfg.PodPort = "11180"
	}
	if cfg.Ns == "" {
		cfg.Ns = "default"
	}
	if cfg.Image == "" {
		out, err := e.k.Kubectl("get deployments.apps -n cpaas-system alb-operator-ctl -o jsonpath='{.spec.template.spec.containers[*].image}'")
		if err != nil {
			return nil, err
		}
		cfg.Image = strings.TrimSpace(out)
	}
	cfg.Raw = strings.TrimSpace(cfg.Raw)
	cfg.Raw = strings.ReplaceAll(cfg.Raw, "	", "  ")
	e.cfg = cfg
	hash_bytes := sha256.Sum256([]byte(cfg.Raw))
	hash_key := hex.EncodeToString(hash_bytes[:])
	// k := e.k
	echo := Template(EchoRestyTemplate, map[string]interface{}{
		"Values": map[string]interface{}{
			"image":          cfg.Image,
			"name":           cfg.Name,
			"ip":             cfg.Ip,
			"replicas":       1,
			"port":           cfg.PodPort,
			"hostport":       cfg.PodHostPort,
			"raw":            cfg.Raw,
			"hash":           hash_key,
			"defaultIngress": cfg.DefaultIngress,
		},
	})
	e.log.Info("yaml", "yaml", echo, "port", cfg.PodPort)
	out, err := e.k.KubectlApply(echo)
	if err != nil {
		return nil, err
	}
	e.log.Info(out)
	// wait and reload nginx. to make sure volume work..
	ctx, _ := context.WithTimeout(context.Background(), time.Second*30)

	err = wait.PollUntilContextTimeout(ctx, time.Second*1, time.Second*3, true,
		func(ctx context.Context) (done bool, err error) {
			pods, err := e.GetRunningPods()
			if err != nil {
				return false, err
			}
			if len(pods) > 0 {
				return true, nil
			}
			return false, nil
		},
	)
	if err != nil {
		return nil, err
	}
	pods, err := e.GetRunningPods()
	if err != nil {
		return nil, err
	}
	pod := pods[0]

	err = wait.PollUntilContextTimeout(ctx, time.Second*1, time.Second*20, true,
		func(ctx context.Context) (done bool, err error) {
			// speed up https://ahmet.im/blog/kubernetes-secret-volumes-delay/
			e.k.Kubectl("annotate", "pod", "--overwrite", "-n", cfg.Ns, pod.GetName(), fmt.Sprintf("update=%d", time.Now().Unix()))
			out, err = e.k.Kubectl("exec", "-n", cfg.Ns, pod.GetName(), "--", "cat", "/etc/nginx/nginx.conf")
			if err != nil {
				e.log.Error(err, "get nginx conf fail")
				return false, nil
			}
			if strings.Contains(out, hash_key) {
				return true, nil
			}
			return false, nil
		},
	)
	if err != nil {
		return nil, err
	}
	_, err = e.k.Kubectl("exec", "-n", cfg.Ns, pod.GetName(), "--", "bash", "-c", "cat /alb/nginx.pid | xargs -I{} kill -HUP {}")
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (e *Echo) GetRunningPods() ([]corev1.Pod, error) {
	pods, err := e.kc.GetPods(e.cfg.Ns, "k8s-app="+e.cfg.Name)
	if err != nil {
		return nil, err
	}
	pods = lo.Filter(pods, func(p corev1.Pod, _ int) bool {
		return p.Status.Phase == "Running"
	})
	return pods, nil
}

func (e *Echo) GetIp() (string, error) {
	ips, err := e.kc.GetPodIp(e.cfg.Ns, "k8s-app="+e.cfg.Name)
	if err != nil {
		return "", err
	}
	ip := ips[0]
	return ip, nil
}

func (e *Echo) GetHostIp() (string, error) {
	pods, err := e.GetRunningPods()
	if err != nil {
		return "", err
	}
	pod := pods[0]
	ip := pod.Status.HostIP
	return ip, nil
}

func (e *Echo) Drop() error {
	return nil
}
