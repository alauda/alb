package controller

import (
	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/thoas/go-funk"
	"strings"
	"sync"
	"time"

	"k8s.io/klog"
)

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
	if lockString == "" {
		return true
	}
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

func TryLockAlbAndUpdateAlbStatus(kd *driver.KubernetesDriver) error {
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
	lockTime := config.GetInt("LOCK_TIMEOUT")
	klog.Infof("locktime %v", lockTime)
	lockString = newLock(myself, time.Duration(lockTime)*time.Second)
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

func GetOwnProjects(name string, labels map[string]string) (rv []string) {
	klog.Infof("get %s own projects %+v", name, labels)
	defer func() {
		klog.Infof("%s, own projects: %+v", name, rv)
	}()
	domain := config.Get("DOMAIN")
	prefix := fmt.Sprintf("project.%s/", domain)
	// legacy: project.cpaas.io/name=ALL_ALL
	// new: project.cpaas.io/ALL_ALL=true
	var projects []string
	for k, v := range labels {
		if strings.HasPrefix(k, prefix) {
			if project := getProjectFromLabel(k, v); project != "" {
				projects = append(projects, project)
			}
		}
	}
	rv = funk.UniqString(projects)
	return
}

const (
	RoleInstance = "instance"
	RolePort     = "port"
)

func GetAlbRoleType(labels map[string]string) string {
	domain := config.Get("DOMAIN")
	roleLabel := fmt.Sprintf("%s/role", domain)
	if labels[roleLabel] == "" || labels[roleLabel] == RoleInstance {
		return RoleInstance
	}
	return RolePort
}

func getProjectFromLabel(k, v string) string {
	domain := config.Get("DOMAIN")
	prefix := fmt.Sprintf("project.%s/", domain)
	if k == fmt.Sprintf("project.%s/name", domain) {
		return v
	} else {
		if v == "true" {
			if project := strings.TrimPrefix(k, prefix); project != "" {
				return project
			}
		}
	}
	return ""
}
