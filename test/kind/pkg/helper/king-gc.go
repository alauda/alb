package helper

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
)

func findNeedDeletedKind(log logr.Logger) ([]string, error) {
	ks, err := KindLs()
	if err != nil {
		return nil, err
	}
	rmks := []string{}
	for _, k := range ks {
		need, reason := isNeedGCKind(k)
		log.Info("check is ned gc kind", "kind", k, "need", need, "reason", reason)
		if need {
			rmks = append(rmks, k)
		}
	}
	return rmks, nil
}

func findNeedDeletedDir(log logr.Logger) ([]string, error) {
	rmDir := []string{}
	base := os.Getenv("ALB_CI_ROOT")
	entries, err := os.ReadDir(base)
	if err != nil {
		log.Error(err, "read base fail")
	}

	for _, e := range entries {
		need, reason := isNeedGCDir(e.Name())
		dir := path.Join(base, e.Name())
		log.Info("check is ned gc dir", "dir", dir, "need", need, "reason", reason)
		if need {
			rmDir = append(rmDir, dir)
		}
	}
	return rmDir, nil
}

func KindGC(log logr.Logger) error {
	if os.Getenv("ALB_CI_ROOT") == "" {
		log.Info("not in ci ignore")
		return nil
	}

	rmKinds, err := findNeedDeletedKind(log)
	if err != nil {
		return err
	}
	rmDrs, err := findNeedDeletedDir(log)
	if err != nil {
		return err
	}
	for _, k := range rmKinds {
		log.Info("rm kind", "kind", k)
		err := KindDelete(k)
		if err != nil {
			log.Error(err, "delete kind fail", "name", k)
		}
	}
	for _, d := range rmDrs {
		log.Info("rm dir", "dir", d)
		err := os.RemoveAll(d)
		if err != nil {
			log.Error(err, "delete dir fail", "name", d)
		}
	}
	return nil
}

func isNeedGCKind(ts string) (bool, string) {
	return isNeedGC(ts)
}

func isNeedGC(ts string) (bool, string) {
	kss := strings.Split(ts, "-")
	if len(kss) < 2 {
		return false, "invalid fmt ignore"
	}
	ts = kss[len(kss)-1]
	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false, fmt.Sprintf("parse int fail %s", err.Error())
	}
	tm := time.Unix(t, 0)
	now := time.Now()
	diff := time.Since(now)
	toleration := time.Hour * 1
	if diff > toleration {
		return true, fmt.Sprintf("over toleration %v | %v | %v", now, tm, diff)
	}
	return false, fmt.Sprintf("in toleration %v | %v | %v", now, tm, diff)
}

func isNeedGCDir(ts string) (bool, string) {
	return isNeedGC(ts)
}
