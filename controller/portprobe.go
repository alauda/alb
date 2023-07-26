package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	"alauda.io/alb2/pkg/apis/alauda/v2beta1"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"

	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type PortProbe struct {
	ctx         context.Context
	kd          *driver.KubernetesDriver
	log         logr.Logger
	cfg         *config.Config
	lister      corev1lister.PodLister
	myPodSel    map[string]string
	listTcpPort func() (map[int]bool, error)
}

func NewPortProbe(ctx context.Context, kd *driver.KubernetesDriver, log logr.Logger, cfg *config.Config) (*PortProbe, error) {
	p := &PortProbe{
		ctx: ctx,
		kd:  kd,
		log: log,
		cfg: cfg,
		myPodSel: map[string]string{
			"service_name": fmt.Sprintf("alb2-%s", cfg.Name),
		},
		listTcpPort: GetListenTCPPorts,
	}

	lister, err := p.initPodClientWithLabel(p.myPodSel, cfg.Ns)
	if err != nil {
		return nil, err
	}
	p.lister = lister
	return p, nil
}

// 统计ft上的异常端口信息到alb的cr上，清理之前旧的pod的异常端口的信息
func (p *PortProbe) LeaderUpdateAlbPortStatus() error {
	cfg := p.cfg
	kd := p.kd
	log := p.log
	//  only update alb when it changed.
	name := cfg.GetAlbName()
	namespace := cfg.GetNs()
	albRes, err := kd.LoadAlbResource(namespace, name)
	if err != nil {
		klog.Errorf("Get alb %s.%s failed: %s", name, namespace, err.Error())
		return err
	}
	fts, err := kd.LoadFrontends(namespace, name)
	if err != nil {
		return err
	}
	dirty, msg, err := p.cleanUpOldPodSatus(fts)
	if err != nil {
		return err
	}
	if dirty {
		log.Info("ft has legacy pod port probe status. wait next time to update alb status", "msg", msg)
		return nil
	}
	status := genCurPortConflictStatus(fts)
	if !albStatusChange(albRes.Status.Detail.Alb, status) {
		return nil
	}
	log.Info("alb status change", "diff", cmp.Diff(albRes.Status.Detail.Alb, status))
	albRes.Status.Detail.Alb = status
	err = kd.UpdateAlbStatus(albRes)
	log.Info("alb status change update success", "ver", albRes.ResourceVersion)
	return err
}

func (p *PortProbe) WorkerDetectAndMaskConflictPort(alb *LoadBalancer) {
	kd := p.kd
	enablePortProbe := p.cfg.GetFlags().EnablePortProbe
	if !enablePortProbe {
		return
	}
	listenTCPPorts, err := p.listTcpPort()
	if err != nil {
		klog.Error(err)
		return
	}
	klog.V(2).Info("finish port probe, listen tcp ports: ", listenTCPPorts)

	for _, ft := range alb.Frontends {
		conflict := false
		if ft.IsTcpBaseProtocol() && listenTCPPorts[int(ft.Port)] {
			conflict = true
			ft.Conflict = true
			klog.Errorf("skip port: %d has conflict", ft.Port)
		}
		if err := p.UpdateFrontendStatus(kd, ft.FtName, conflict); err != nil {
			klog.Error(err)
		}
	}
}

func genCurPortConflictStatus(fts []*alb2v1.Frontend) v2beta1.AlbStatus {
	status := v2beta1.AlbStatus{
		PortStatus: map[string]v2beta1.PortStatus{},
	}
	for _, ft := range fts {
		if ft.Status.Instances == nil {
			continue
		}
		conflictIns := []string{}
		for name, v := range ft.Status.Instances {
			if v.Conflict {
				conflictIns = append(conflictIns, name)
			}
		}
		sort.Strings(conflictIns)
		if len(conflictIns) != 0 {
			key := fmt.Sprintf("%v-%v", ft.Spec.Protocol, ft.Spec.Port)
			msg := fmt.Sprintf("confilct on %s", strings.Join(conflictIns, ", "))
			status.PortStatus[key] = v2beta1.PortStatus{
				Msg:          msg,
				Conflict:     true,
				ProbeTimeStr: metav1.Time{Time: time.Now()},
			}
		}
	}
	return status
}

func albStatusChange(origin, new v2beta1.AlbStatus) bool {
	if len(origin.PortStatus) != len(new.PortStatus) {
		return true
	}
	for key, op := range origin.PortStatus {
		np, find := new.PortStatus[key]
		if !find {
			return true
		}
		if np.Conflict != op.Conflict || np.Msg != op.Msg {
			return true
		}
	}
	return false
}

func genPodPortConflictKey(host string, pod string) string {
	return host + "/" + pod
}
func parsePodPortConflictKey(key string) (host string, pod string, err error) {
	items := strings.Split(key, "/")
	if len(items) != 2 {
		return "", "", fmt.Errorf("invalid format of port conflict key %s", key)
	}
	return items[0], items[1], nil
}

func (p *PortProbe) UpdateFrontendStatus(kd *driver.KubernetesDriver, ftName string, conflictState bool) error {
	ft, err := kd.FrontendLister.Frontends(p.cfg.GetNs()).Get(ftName)
	if err != nil {
		return err
	}
	origin := ft.DeepCopy()
	hostname, err := os.Hostname()
	key := genPodPortConflictKey(hostname, p.cfg.Controller.PodName)
	if err != nil {
		return err
	}
	if ft.Status.Instances == nil {
		ft.Status.Instances = make(map[string]alb2v1.Instance)
	}

	preConflictState := false
	if instance, ok := ft.Status.Instances[key]; ok {
		preConflictState = instance.Conflict
	}

	if preConflictState == conflictState {
		return nil
	}

	ft.Status.Instances[key] = alb2v1.Instance{
		Conflict:  conflictState,
		ProbeTime: time.Now().Unix(),
	}
	return p.patchFtStatus(origin, ft)
}

func (p *PortProbe) patchFtStatus(old *alb2v1.Frontend, new *alb2v1.Frontend) error {
	ns := p.cfg.GetNs()
	ctx := p.ctx
	bytesOrigin, err := json.Marshal(old)
	if err != nil {
		return err
	}
	bytesModified, err := json.Marshal(new)
	if err != nil {
		return err
	}
	patch, err := jsonpatch.CreateMergePatch(bytesOrigin, bytesModified)
	if err != nil {
		return err
	}
	if string(patch) == "{}" {
		return nil
	}
	klog.Infof("patch ft status %v", string(patch))
	if _, err := p.kd.ALBClient.CrdV1().Frontends(ns).Patch(ctx, new.Name, types.MergePatchType, patch, metav1.PatchOptions{}, "status"); err != nil {
		return err
	}
	return nil
}

var (
	excludeProcess = map[string]bool{
		"nginx":      true,
		"nginx.conf": true,
	}
	// users:(("nginx",pid=31486,fd=8),("nginx",pid=31485,fd=8))
	processPattern = regexp.MustCompile(`\("(.*?)",pid=.*?\)`)
)

func GetListenTCPPorts() (map[int]bool, error) {
	//	/ # ss -ntlp
	//	State                           Recv-Q                          Send-Q                                                     Local Address:Port                                                      Peer Address:Port
	//	LISTEN                          0                               128                                                                 [::]:22                                                                [::]:*
	raw, err := exec.Command("ss", "-ntlp").CombinedOutput()
	if err != nil {
		return nil, err
	}
	ports := map[int]bool{}
	output := strings.TrimSpace(string(raw))
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		for _, line := range lines {
			if !strings.Contains(line, "LISTEN") {
				continue
			}
			fields := strings.Fields(line)
			rawLocalAddr := fields[3]
			t := strings.Split(rawLocalAddr, ":")
			port, err := strconv.Atoi(t[len(t)-1])
			if err != nil {
				continue
			}
			processName := "-"
			if len(fields) == 6 {
				rawProcess := fields[5]
				t = processPattern.FindStringSubmatch(rawProcess)
				if len(t) >= 2 {
					processName = t[1]
				}
			}
			if !excludeProcess[processName] {
				ports[port] = true
			}
		}
	}
	return ports, nil
}

func (p *PortProbe) initPodClientWithLabel(podLables map[string]string, ns string) (corev1lister.PodLister, error) {
	labelSelector := labels.Set(podLables).AsSelector()

	filteredFactory := informers.NewSharedInformerFactoryWithOptions(p.kd.Client, 0, informers.WithNamespace(ns), informers.WithTweakListOptions(func(options *metav1.ListOptions) {
		options.LabelSelector = labelSelector.String()
	}))
	filteredFactory.Start(p.ctx.Done())

	pods := filteredFactory.Core().V1().Pods()
	podInformer := pods.Informer()
	filteredFactory.Start(p.ctx.Done())
	ok := cache.WaitForNamedCacheSync("portprobe", p.ctx.Done(), podInformer.HasSynced)
	if !ok {
		return nil, fmt.Errorf("init portprobe client fail ")
	}
	return pods.Lister(), nil
}

func (p *PortProbe) getAlbPod() (sets.Set[string], error) {
	pods, err := p.lister.List(labels.Set(p.myPodSel).AsSelector())
	if err != nil {
		return nil, err
	}
	return sets.New(lo.Map(pods, func(p *corev1.Pod, _ int) string { return p.Name })...), nil
}

func (p *PortProbe) cleanUpOldPodSatus(fts []*alb2v1.Frontend) (dirty bool, msg string, err error) {
	curPods, err := p.getAlbPod()
	if err != nil {
		return false, "", err
	}
	reconcile := false
	msg = ""
	for _, ft := range fts {
		ftDirty := false
		origin := ft.DeepCopy()
		for key, _ := range ft.Status.Instances {
			_, pod, err := parsePodPortConflictKey(key)
			if err != nil || !curPods.Has(pod) {
				ftDirty = true
				reconcile = true
				msg = msg + " " + key
				delete(ft.Status.Instances, key)
			}
		}
		if ftDirty {
			p.log.Info("find dirty", "key", msg, "ft", ft.Name)
			err := p.patchFtStatus(origin, ft)
			if err != nil {
				p.log.Error(err, "clean up pod status fail")
			}
		}
	}
	return reconcile, msg, nil
}
