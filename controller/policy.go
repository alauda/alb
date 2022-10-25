package controller

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"os"

	"alauda.io/alb2/config"
	"k8s.io/klog/v2"
)

func (nc *NginxController) updatePolicyFileRaw(ngxPolicies NgxPolicy, path string, zip bool) error {
	policyBytes, err := json.MarshalIndent(ngxPolicies, "", "\t")
	if err != nil {
		klog.Error()
		return err
	}
	policyWriter, err := os.Create(path)
	if err != nil {
		klog.Errorf("Failed to create new policy file %s", err.Error())
		return err
	}
	err = policyWriter.Chmod(0640)
	if err != nil {
		klog.Errorf("Failed to add read permission of %s, err %v", path, err.Error(), err)
		return err
	}
	defer policyWriter.Close()
	if zip {
		var b bytes.Buffer
		w := zlib.NewWriter(&b)
		_, err := w.Write(policyBytes)
		if err != nil {
			klog.Errorf("zip policy fail %v", err)
			return err
		}
		err = w.Close()
		if err != nil {
			klog.Errorf("zip policy fail %v", err)
			return err
		}
		policyBytes = b.Bytes()
	}
	if _, err := policyWriter.Write(policyBytes); err != nil {
		klog.Errorf("Write policy file failed %s", err.Error())
		return err
	}
	policyWriter.Sync()
	return nil
}

func (nc *NginxController) UpdatePolicyFile(ngxPolicies NgxPolicy) error {
	zip := config.GetBool("POLICY_ZIP")
	path := nc.NewPolicyPath
	if zip {
		path = path + ".bin"
	}
	klog.Infof("update policy %v", path)
	return nc.updatePolicyFileRaw(ngxPolicies, path, zip)
}
