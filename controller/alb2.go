package controller

import (
	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	m "alauda.io/alb2/modules"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog"
)

func MergeNew(alb *m.AlaudaLoadBalancer) (*LoadBalancer, error) {
	lb := &LoadBalancer{
		Name:           alb.Name,
		Address:        alb.Spec.Address,
		BindAddress:    alb.Spec.BindAddress,
		LoadBalancerID: alb.Spec.IaasID,
		Frontends:      []*Frontend{},
		TweakHash:      alb.TweakHash,
	}
	if lb.BindAddress == "" {
		lb.BindAddress = "*"
	}
	for _, aft := range alb.Frontends {
		ft := &Frontend{
			RawName:         aft.Name,
			LoadBalancerID:  alb.Name,
			Port:            aft.Port,
			Protocol:        aft.Protocol,
			Rules:           RuleList{},
			CertificateName: aft.CertificateName,
		}
		if ft.Protocol == "" {
			ft.Protocol = ProtocolTCP
		}
		if ft.Port <= 0 {
			klog.Errorf("frontend %s has an invalid port %d", aft.Name, aft.Port)
		}
		for _, arl := range aft.Rules {
			rule := &Rule{
				RuleID:          arl.Name,
				Priority:        arl.Priority,
				Type:            arl.Type,
				Domain:          arl.Domain,
				URL:             arl.URL,
				DSL:             arl.DSL,
				DSLX:            arl.DSLX,
				Description:     arl.Description,
				CertificateName: arl.CertificateName,
				RewriteTarget:   arl.RewriteTarget,
				EnableCORS:      arl.EnableCORS,
				BackendProtocol: arl.BackendProtocol,
				RedirectURL:     arl.RedirectURL,
				RedirectCode:    arl.RedirectCode,
				VHost:           arl.VHost,
			}
			if arl.ServiceGroup != nil {
				rule.SessionAffinityPolicy = arl.ServiceGroup.SessionAffinityPolicy
				rule.SessionAffinityAttr = arl.ServiceGroup.SessionAffinityAttribute
				for _, svc := range arl.ServiceGroup.Services {
					if rule.Services == nil {
						rule.Services = []*BackendService{}
					}
					rule.Services = append(rule.Services, &BackendService{
						ServiceID:     svc.ServiceID(),
						ContainerPort: svc.Port,
						Weight:        svc.Weight,
					})
				}
			}
			ft.Rules = append(ft.Rules, rule)
		}
		if aft.ServiceGroup != nil {
			ft.Services = []*BackendService{}
			ft.BackendGroup = &BackendGroup{
				Name:                     ft.String(),
				SessionAffinityAttribute: aft.ServiceGroup.SessionAffinityAttribute,
				SessionAffinityPolicy:    aft.ServiceGroup.SessionAffinityPolicy,
			}
			for _, svc := range aft.ServiceGroup.Services {
				ft.Services = append(ft.Services, &BackendService{
					ServiceID:     svc.ServiceID(),
					ContainerPort: svc.Port,
					Weight:        svc.Weight,
				})
			}
		}

		lb.Frontends = append(lb.Frontends, ft)
	}
	return lb, nil
}

var ErrAlbInUse = errors.New("alb2 is used by another controller")

type Lock struct {
	Owner     string
	LockUntil time.Time
}

func newLock(ownerID string, timeout time.Duration) string {
	lock := Lock{
		Owner:     ownerID,
		LockUntil: time.Now().Add(timeout),
	}
	lockString, err := json.Marshal(lock)
	if err != nil {
		panic(err)
	}
	return string(lockString)
}

func locked(lockString, ownerID string) bool {
	var lock Lock
	err := json.Unmarshal([]byte(lockString), &lock)
	if err != nil {
		klog.Error(err)
		return false
	}
	now := time.Now()
	if now.Before(lock.LockUntil) && lock.Owner != ownerID {
		//lock by another
		return true
	}
	return false
}

func needRelock(lockString string) bool {
	var lock Lock
	err := json.Unmarshal([]byte(lockString), &lock)
	if err != nil {
		klog.Error(err)
		return true
	}
	if lock.LockUntil.Sub(time.Now()) <= 30*time.Second {
		return true
	}

	return false
}

func retryUntil(lockString string) time.Time {
	var lock Lock
	err := json.Unmarshal([]byte(lockString), &lock)
	if err != nil {
		klog.Error(err)
		return time.Now()
	}
	return lock.LockUntil.Add(-30 * time.Second)
}

var myself string
var mutexLock sync.Mutex
var holdUntil time.Time
var waitUntil time.Time

func TryLockAlb(kd *driver.KubernetesDriver) error {
	now := time.Now()
	klog.Infof("try lock alb, now: %s, holdUntil: %s, waitUntil: %s", now, holdUntil, waitUntil)
	mutexLock.Lock()
	defer mutexLock.Unlock()
	if myself == "" {
		if config.Get("MY_POD_NAME") != "" {
			myself = config.Get("MY_POD_NAME")
		} else {
			myself = RandomStr("", 8)
		}
	}
	if now.Before(holdUntil) {
		return nil
	}
	if now.Before(waitUntil) {
		return ErrAlbInUse
	}
	name := config.Get("NAME")
	namespace := config.Get("NAMESPACE")
	albRes, err := kd.LoadAlbResource(namespace, name)
	if err != nil {
		klog.Errorf("Get alb %s.%s failed: %s", name, namespace, err.Error())
		return err
	}
	if albRes.Annotations == nil {
		albRes.Annotations = make(map[string]string)
	}
	lockString, ok := albRes.Annotations[fmt.Sprintf(config.Get("labels.lock"), config.Get("DOMAIN"))]
	klog.Infof("lockstring: %s", lockString)
	if ok && locked(lockString, myself) {
		// used by another pod of alb2
		waitUntil = retryUntil(lockString)
		klog.Info(ErrAlbInUse)
		return ErrAlbInUse
	}

	if !needRelock(lockString) {
		klog.Info("Hold lock, no need to relock")
		return nil
	}

	lockString = newLock(myself, 90*time.Second)
	albRes.Annotations[fmt.Sprintf(config.Get("labels.lock"), config.Get("DOMAIN"))] = lockString
	fts, err := kd.LoadFrontends(namespace, name)
	if err != nil {
		return err
	}
	state := "ready"
	reason := ""
	for _, ft := range fts {
		if ft.Status.Instances != nil {
			for _, v := range ft.Status.Instances {
				if v.Conflict == true {
					state = "warning"
					reason = "port conflict"
					break
				}
			}
		}
	}
	albRes.Status.State = state
	albRes.Status.Reason = reason
	albRes.Status.ProbeTime = time.Now().Unix()
	err = kd.UpdateAlbResource(albRes)
	if err != nil {
		klog.Errorf("lock %s.%s failed: %s", name, namespace, err.Error())
		return err
	}
	klog.Infof("I locked alb.")
	holdUntil = retryUntil(lockString)
	return nil
}

func IsLocker(kd *driver.KubernetesDriver) (bool, error) {
	if myself == "" {
		if config.Get("MY_POD_NAME") != "" {
			myself = config.Get("MY_POD_NAME")
		} else {
			myself = RandomStr("", 8)
		}
	}
	name := config.Get("NAME")
	namespace := config.Get("NAMESPACE")
	albRes, err := kd.LoadAlbResource(namespace, name)
	if err != nil {
		klog.Errorf("Get alb %s.%s failed: %s", name, namespace, err.Error())
		return false, err
	}
	if albRes.Annotations == nil {
		albRes.Annotations = make(map[string]string)
	}
	lockString, ok := albRes.Annotations[fmt.Sprintf(config.Get("labels.lock"), config.Get("DOMAIN"))]
	if ok {
		if !locked(lockString, myself) {
			return true, nil
		}
		return false, ErrAlbInUse
	}
	return false, nil
}
