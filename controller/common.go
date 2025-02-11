package controller

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"alauda.io/alb2/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pkgcfg "alauda.io/alb2/pkg/config"
	"k8s.io/klog/v2"
)

var (
	SUCCESS = "success"
	FAILED  = "failed"
)

func sameFiles(file1, file2 string) bool {
	sum1, err := fileSha256(file1)
	if err != nil {
		klog.Warning(err.Error())
		return false
	}
	sum2, err := fileSha256(file2)
	if err != nil {
		klog.Warning(err.Error())
		return false
	}

	return sum1 == sum2
}

func fileSha256(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	sha256h := sha256.New()
	_, err = io.Copy(sha256h, f)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256h.Sum(nil)), nil
}

func reverseStatus(status string) string {
	if status == SUCCESS {
		return FAILED
	}
	return SUCCESS
}

func setLastReloadStatus(status, statusFileParentPath string) error {
	statusFilePath := path.Join(statusFileParentPath, status)
	if _, err := os.Stat(statusFilePath); os.IsNotExist(err) {
		f, err := os.Create(statusFilePath)
		if err != nil {
			klog.Errorf("create new status file failed %s", err.Error())
			return err
		}
		f.Close()
	}

	reversStatusFilePath := path.Join(statusFileParentPath, reverseStatus(status))
	if _, err := os.Stat(reversStatusFilePath); err == nil {
		err := os.Remove(reversStatusFilePath)
		if err != nil {
			klog.Errorf("remove old status file failed %s", err.Error())
			return err
		}
	}
	return nil
}

func getLastReloadStatus(statusFileParentPath string) string {
	successStatusFilePath := path.Join(statusFileParentPath, SUCCESS)
	if _, err := os.Stat(successStatusFilePath); err == nil {
		klog.Infof("last reload status: %s", SUCCESS)
		return SUCCESS
	}
	klog.Info("last reload status", FAILED)
	return FAILED
}

func getPortInfo(driver *driver.KubernetesDriver, ns string, name string) (map[string][]string, error) {
	cm, err := driver.Client.CoreV1().ConfigMaps(ns).Get(driver.Ctx, fmt.Sprintf("%s-port-info", name), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if cm.Data["range"] != "" {
		var body pkgcfg.PortProject
		if err := json.Unmarshal([]byte(cm.Data["range"]), &body); err != nil {
			return nil, err
		}
		rv := make(map[string][]string)
		for _, i := range body {
			rv[i.Port] = i.Projects
		}
		return rv, nil
	}
	return nil, fmt.Errorf("invalid port-info format, %v", cm.Data)
}

func getPortProject(port int, info map[string][]string) ([]string, error) {
	for k, v := range info {
		if strings.Contains(k, "-") {
			// port range
			parts := strings.Split(k, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid port range %s", k)
			}
			s, e := parts[0], parts[1]
			start, err := strconv.Atoi(s)
			if err != nil {
				return nil, err
			}
			end, err := strconv.Atoi(e)
			if err != nil {
				return nil, err
			}
			if start >= end {
				return nil, errors.New("ip range start should less than end")
			}
			if port >= start && port <= end {
				return v, nil
			}
		} else {
			// single port
			single, err := strconv.Atoi(k)
			if err != nil {
				return nil, err
			}
			if single == port {
				return v, nil
			}
		}
	}
	return nil, nil
}

func generatePatchPortProjectPayload(labels map[string]string, desiredProjects []string, domain string) []byte {
	newLabels := make(map[string]string)
	// project.cpaas.io/ALL_ALL=true
	for k, v := range labels {
		if !strings.HasPrefix(k, fmt.Sprintf("project.%s", domain)) {
			newLabels[k] = v
		}
	}
	for _, p := range desiredProjects {
		newLabels[fmt.Sprintf("project.%s/%s", domain, p)] = "true"
	}
	patchPayloadTemplate := `[{
        "op": "%s",
        "path": "/metadata/labels",
        "value": %s
          }]`

	raw, _ := json.Marshal(newLabels) //nolint:errcheck
	return []byte(fmt.Sprintf(patchPayloadTemplate, "replace", raw))
}
