package framework

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"
)

type AlbWaitFileExt struct {
	reader ReadAlbFile
	alb    AlbInfo
	log    logr.Logger
}

type DefaultReadFile struct{}

func (d *DefaultReadFile) ReadFile(p string) (string, error) {
	ret, err := os.ReadFile(p)
	return string(ret), err
}

type ReadAlbFile interface {
	ReadFile(p string) (string, error)
}

func NewAlbWaitFileExt(reader ReadAlbFile, albInfo AlbInfo, log logr.Logger) *AlbWaitFileExt {
	return &AlbWaitFileExt{
		reader: reader,
		alb:    albInfo,
		log:    log,
	}
}

func (f *AlbWaitFileExt) waitFile(file string, matcher func(string) (bool, error)) {
	err := wait.Poll(Poll, DefaultTimeout, func() (bool, error) {
		fileCtx, err := f.reader.ReadFile(file)
		if err != nil {
			return false, nil
		}
		ok, err := matcher(fileCtx)
		if err != nil {
			return false, err
		}
		return ok, nil
	})
	assert.Nil(ginkgo.GinkgoT(), err, "wait nginx config contains fail")
}

func regexMatch(text string, matchStr string) bool {
	match, _ := regexp.MatchString(matchStr, text)
	return match
}

func (f *AlbWaitFileExt) WaitNginxConfig(check func(raw string) (bool, error)) {
	p := f.alb.NginxCfgPath
	f.waitFile(p, check)
}

func (f *AlbWaitFileExt) WaitNginxConfigStr(regexStr string) {
	p := f.alb.NginxCfgPath
	f.WaitNginxConfig(func(raw string) (bool, error) {
		match := regexMatch(raw, regexStr)
		f.log.Info("match nginx confg regex", "str", regexStr, "path", p, "match", match)
		return match, nil
	})
}

func (f *AlbWaitFileExt) WaitPolicyRegex(regexStr string) {
	p := f.alb.PolicyPath
	f.waitFile(p, func(raw string) (bool, error) {
		match := regexMatch(raw, regexStr)
		f.log.Info("match policy regex", "str", regexStr, "path", p, "match", match)
		return match, nil
	})
}

func (f *AlbWaitFileExt) WaitNgxPolicy(fn func(p NgxPolicy) (bool, error)) {
	p := f.alb.PolicyPath
	f.waitFile(p, func(raw string) (bool, error) {
		Logf("p %s  %s", p, raw)
		p := NgxPolicy{}
		err := json.Unmarshal([]byte(raw), &p)
		if err != nil {
			return false, fmt.Errorf("wait nginx policy fial err %v raw -- %s --", err, raw)
		}
		return TestEq(func() bool {
			ret, err := fn(p)
			if err != nil {
				Logf("test eq find err %v", err)
				return false
			}
			return ret
		}), nil
	})
}

func (f *AlbWaitFileExt) WaitPolicy(fn func(raw string) bool) {
	p := f.alb.PolicyPath
	f.waitFile(p, func(raw string) (bool, error) {
		match := fn(raw)
		Logf("match in %s %v", p, match)
		return match, nil
	})
}
