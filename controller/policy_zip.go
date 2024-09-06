package controller

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"os"

	. "alauda.io/alb2/controller/types"

	"k8s.io/klog/v2"
)

func (nc *NginxController) UpdatePolicyFile(ngxPolicies NgxPolicy) error {
	zip := nc.albcfg.GetFlags().PolicyZip
	path := nc.NewPolicyPath
	if zip {
		path += ".bin"
	}
	klog.Infof("update policy %v", path)
	return nc.updatePolicyFileRaw(ngxPolicies, path, zip)
}

func (nc *NginxController) updatePolicyFileRaw(ngxPolicies NgxPolicy, path string, zip bool) error {
	oldpath := path + ".old"
	policyBytes, err := json.MarshalIndent(ngxPolicies, "", "\t")
	if err != nil {
		klog.Error()
		return err
	}
	policyWriter, err := os.Create(oldpath)
	if err != nil {
		klog.Errorf("Failed to create new policy file %s", err.Error())
		return err
	}
	err = policyWriter.Chmod(0o640)
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
	err = policyWriter.Sync()
	if err != nil {
		return err
	}
	return os.Rename(oldpath, path)
}
