package controller

import (
	"errors"
	"fmt"
	"strings"

	"github.com/thoas/go-funk"

	alb2v2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"k8s.io/klog/v2"
)

var ErrAlbInUse = errors.New("alb2 is used by another controller")

func GetOwnProjectsFromLabel(name string, labels map[string]string, domain string) (rv []string) {
	defer func() {
		klog.Infof("own projects from labels. labels: %v name: %s projects: %v", labels, name, rv)
	}()
	prefix := fmt.Sprintf("project.%s/", domain)
	// legacy: project.cpaas.io/name=ALL_ALL
	// new: project.cpaas.io/ALL_ALL=true
	var projects []string
	for k, v := range labels {
		if strings.HasPrefix(k, prefix) {
			if project := getProjectFromLabel(k, v, domain); project != "" {
				projects = append(projects, project)
			}
		}
	}
	rv = funk.UniqString(projects)
	return
}

func GetOwnProjectsFromAlb(alb *alb2v2.ALB2) (rv []string) {
	projects := []string{}
	if alb != nil && alb.Spec.Config != nil && alb.Spec.Config.Projects != nil {
		projects = alb.Spec.Config.Projects
	}
	return projects
}

const (
	RoleInstance = "instance"
	RolePort     = "port"
)

func GetAlbRoleType(labels map[string]string, domain string) string {
	roleLabel := fmt.Sprintf("%s/role", domain)
	if labels[roleLabel] == "" || labels[roleLabel] == RoleInstance {
		return RoleInstance
	}
	return RolePort
}

func getProjectFromLabel(k, v, domain string) string {
	prefix := fmt.Sprintf("project.%s/", domain)
	if k == fmt.Sprintf("project.%s/name", domain) {
		return v
	} else if v == "true" {
		if project := strings.TrimPrefix(k, prefix); project != "" {
			return project
		}
	}
	return ""
}
