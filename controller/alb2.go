package controller

import (
	"errors"
	"fmt"
	"strings"

	"alauda.io/alb2/config"
	"github.com/thoas/go-funk"

	"k8s.io/klog/v2"
)

var ErrAlbInUse = errors.New("alb2 is used by another controller")

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
