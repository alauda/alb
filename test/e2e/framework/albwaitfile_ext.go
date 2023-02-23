package framework

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"
)

type AlbWaitFileExt struct {
	reader ReadAlbFile
	alb    AlbInfo
}

func NewAlbWaitFileExt(reader ReadAlbFile, albInfo AlbInfo) *AlbWaitFileExt {
	return &AlbWaitFileExt{
		reader: reader,
		alb:    albInfo,
	}
}

func (f *AlbWaitFileExt) waitFile(file string, matcher func(string) (bool, error)) {
	err := wait.Poll(Poll, DefaultTimeout, func() (bool, error) {
		fileCtx, err := f.reader.ReadFile(file)
		if err != nil {
			return false, nil
		}
		ok, err := matcher(string(fileCtx))
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
		Logf("match regex %s in %s %v", regexStr, p, match)
		return match, nil
	})
}

func (f *AlbWaitFileExt) WaitPolicyRegex(regexStr string) {
	p := f.alb.PolicyPath
	f.waitFile(p, func(raw string) (bool, error) {
		match := regexMatch(raw, regexStr)
		Logf("match regex %s in %s %v", regexStr, p, match)
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
