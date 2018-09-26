package controller

import (
	"alb2/driver"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/golang/glog"
)

type HaproxyController struct {
	BackendType   string
	TemplatePath  string
	NewConfigPath string
	OldConfigPath string
	BinaryPath    string
	Driver        *driver.KubernetesDriver
}

func (hc *HaproxyController) GetLoadBalancerType() string {
	return "Haproxy"
}

func (hc *HaproxyController) GenerateConf() error {

	writer, err := os.Create(hc.NewConfigPath)
	if err != nil {
		glog.Errorf("Failed to create new config %s", err.Error())
		return err
	}
	defer writer.Close()

	t, err := template.New("haproxy.tmpl").ParseFiles(hc.TemplatePath)
	if err != nil {
		glog.Errorf("Failed to parse template %s", err.Error())
		return err
	}

	services, err := hc.Driver.ListService()
	if err != nil {
		return err
	}
	loadbalancers, err := FetchLoadBalancersInfo()
	if err != nil {
		return err
	}

	merge(loadbalancers, services)
	if err != nil {
		return err
	}

	if len(loadbalancers[0].Frontends) == 0 {
		glog.Info("No service bind to this haproxy now")
	}

	haproxyConfig := generateConfig(loadbalancers[0])

	err = t.Execute(writer, haproxyConfig)
	if err != nil {
		glog.Error(err)
	}
	writer.Sync()
	return err
}

func (hc *HaproxyController) ReloadLoadBalancer() error {
	if hc.BinaryPath == "" {
		glog.Info("haproxy bin path is empty!")
		return nil
	}
	var err error
	if hc.BinaryPath == "test" {
		// for local test
		return err
	}

	defer func() {
		if err != nil {
			setLastReloadStatus(FAILED, StatusFileParentPath)
		} else {
			setLastReloadStatus(SUCCESS, StatusFileParentPath)
		}
	}()

	pids, err := CheckProcessAlive(hc.BinaryPath)
	if err != nil && err.Error() != "exit status 1" {
		glog.Errorf("failed to check haproxy aliveness: %s", err.Error())
		return err
	}
	pids = strings.Trim(pids, "\n ")

	configChanged := !sameFiles(hc.NewConfigPath, hc.OldConfigPath)
	if !configChanged && len(pids) > 0 && getLastReloadStatus(StatusFileParentPath) == SUCCESS {
		glog.Info("Config not changed and last reload success")
		return nil
	}

	if configChanged {
		diffOutput, _ := exec.Command("diff", "-u", hc.OldConfigPath, hc.NewConfigPath).CombinedOutput()
		glog.Infof("Haproxy configuration diff\n")
		glog.Infof("%v\n", string(diffOutput))

		glog.Info("Start to change config.")
		err = os.Rename(hc.NewConfigPath, hc.OldConfigPath)
		if err != nil {
			glog.Errorf("failed to replace config: %s", err.Error())
			return err
		}
	}

	if len(pids) == 0 {
		err = hc.start()
	} else {
		err = hc.reload(pids)
	}

	return err
}

func (hc *HaproxyController) start() error {
	glog.Infof("Run command %s -f %s", hc.BinaryPath, hc.OldConfigPath)
	output, err := exec.Command(hc.BinaryPath, "-f", hc.OldConfigPath).CombinedOutput()
	if err != nil {
		glog.Errorf("start haproxy failed: %s ,%v", output, err)
	}
	return err
}

func (hc *HaproxyController) reload(pids string) error {

	args := []string{"-f", hc.OldConfigPath, "-sf"}
	args = append(args, strings.Split(pids, "\n")...)
	output, err := exec.Command(hc.BinaryPath, args...).CombinedOutput()
	if err != nil {
		glog.Errorf("reload haproxy failed: %s %v", output, err)
		return err
	}
	return nil
}
