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
		Address:        alb.Address,
		BindAddress:    alb.BindAddress,
		LoadBalancerID: alb.LoadBalancerID,
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
			CertificateID:   aft.CertificateID,
			CertificateName: aft.CertificateName,
			Rules:           RuleList{},
		}
		if ft.Protocol == "" {
			ft.Protocol = ProtocolTCP
		}
		if ft.Port <= 0 {
			glog.Errorf("frontend %s has an invalid port %d", aft.Name, aft.Port)
		}
		for idx, arl := range aft.Rules {
			rule := &Rule{
				RuleID:      arl.Name,
				Priority:    arl.Priority * int64(idx+1),
				Type:        arl.Type,
				Domain:      arl.Domain,
				URL:         arl.URL,
				DSL:         arl.DSL,
				Description: arl.Description,
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

var myself string

func TryLockAlb() error {
	if myself == "" {
		myself = RandomStr("", 8)
	}
	name := config.Get("NAME")
	namespace := config.Get("NAMESPACE")
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
		return errors.New("alb is in use")
	}
	albRes.Annotations[config.Get("labels.lock")] = newLock(myself, 30*time.Second)
	err = driver.UpdateAlbResource(albRes)
	if err != nil {
		glog.Errorf("lock %s.%s failed: %s", name, namespace, err.Error())
		return err
	}
	return nil
}
