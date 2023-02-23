package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
)

func backupkey(domain string) string {
	return fmt.Sprintf("alb.%s/migrate-backup", domain)
}

func (r *ALB2Reconciler) IsV2Alb(alb *albv2.ALB2) bool {
	key := backupkey(r.Env.BaseDomain)
	backup := alb.Annotations[key]
	if alb.Spec.Config != nil || backup != "" {
		return true
	}
	return false
}

func (r *ALB2Reconciler) IsNormalV2Alb(alb *albv2.ALB2) bool {
	key := backupkey(r.Env.BaseDomain)
	backup := alb.Annotations[key]
	if alb.Spec.Config != nil || backup != "" {
		return true
	}
	return false
}

func (r *ALB2Reconciler) HandleBackupAnnotation(ctx context.Context, alb *albv2.ALB2) (requeue bool, err error) {
	if alb.Spec.Config != nil {
		return false, nil
	}
	key := backupkey(r.Env.BaseDomain)
	backup := alb.Annotations[key]
	if backup == "" {
		return false, fmt.Errorf("backup is empty")
	}

	r.Log.Info("find backup. config is nill", "alb", alb.Name, "backup", backup)
	cfg := &albv2.ExternalAlbConfig{}
	err = json.Unmarshal([]byte(backup), cfg)
	if err != nil {
		return false, err
	}
	alb.Spec.Config = cfg
	err = r.Update(ctx, alb)
	if err != nil {
		return false, err
	}
	r.Log.Info("migrate backup to config", "alb", alb.Name, "backup", backup, "version", alb.ResourceVersion)
	return true, err
}
