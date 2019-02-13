package controller

import (
	"alb2/config"
	"alb2/driver"
	m "alb2/modules"
	"encoding/json"
	"errors"
	"time"

	"github.com/golang/glog"
)

func MergeNew(alb *m.AlaudaLoadBalancer) (*LoadBalancer, error) {
	lb := &LoadBalancer{
		Name:           alb.Name,
		Address:        alb.Spec.Address,
		BindAddress:    alb.Spec.BindAddress,
		LoadBalancerID: alb.Spec.IaasID,
		Frontends:      []*Frontend{},
	}
	if lb.BindAddress == "" {
		lb.BindAddress = "*"
	}
	for _, aft := range alb.Frontends {
		ft := &Frontend{
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
			glog.Errorf("frontend %s has an invalid port %d", aft.Name, aft.Port)
		}
		for idx, arl := range aft.Rules {
			rule := &Rule{
				RuleID:          arl.Name,
				Priority:        int64(arl.Priority) * int64(idx+1),
				Type:            arl.Type,
				Domain:          arl.Domain,
				URL:             arl.URL,
				DSL:             arl.DSL,
				Description:     arl.Description,
				CertificateName: arl.CertificateName,
				RewriteTarget:   arl.RewriteTarget,
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
			for _, svc := range aft.ServiceGroup.Services {
				ft.ServiceID = svc.ServiceID()
				ft.ContainerPort = svc.Port
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
		glog.Error(err)
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
		glog.Error(err)
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
		glog.Error(err)
		return time.Now()
	}
	return lock.LockUntil.Add(-30 * time.Second)
}

var myself string
var holdUntil time.Time
var waitUntil time.Time

func TryLockAlb() error {
	if myself == "" {
		myself = RandomStr("", 8)
	}
	now := time.Now()
	if now.Before(holdUntil) {
		return nil
	}
	if now.Before(waitUntil) {
		return ErrAlbInUse
	}
	name := config.Get("NAME")
	namespace := config.Get("NAMESPACE")
	driver, err := driver.GetDriver()
	if err != nil {
		return err
	}
	albRes, err := driver.LoadAlbResource(namespace, name)
	if err != nil {
		glog.Errorf("Get alb %s.%s failed: %s", name, namespace, err.Error())
		return err
	}
	if albRes.Annotations == nil {
		albRes.Annotations = make(map[string]string)
	}
	lockString, ok := albRes.Annotations[config.Get("labels.lock")]
	if ok && locked(lockString, myself) {
		// used by another pod of alb2
		waitUntil = retryUntil(lockString)
		glog.Info(ErrAlbInUse)
		return ErrAlbInUse
	}

	if !needRelock(lockString) {
		glog.Info("Hold lock, no need to relock")
		return nil
	}

	lockString = newLock(myself, 90*time.Second)
	albRes.Annotations[config.Get("labels.lock")] = lockString
	err = driver.UpdateAlbResource(albRes)
	if err != nil {
		glog.Errorf("lock %s.%s failed: %s", name, namespace, err.Error())
		return err
	}
	glog.Infof("I locked alb.")
	holdUntil = retryUntil(lockString)
	return nil
}
