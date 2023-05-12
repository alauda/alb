package helper

import (
	"strings"
	"unicode"

	. "alauda.io/alb2/utils/test_utils"
)

// 了解alb的chart的内容，给定一个alb的chart，可以从中获取信息

type AlbChartExt struct {
	Base string
	helm *Helm
}

func NewAlbChart(helm *Helm) *AlbChartExt {
	return &AlbChartExt{
		helm: helm,
	}
}

func (a *AlbChartExt) Pull(chart string) (string, error) {
	h := a.helm
	return h.Pull(chart)
}

func (a *AlbChartExt) ListImage(chart string) ([]string, error) {
	registry := "registry.alauda.cn:60080"
	h := a.helm
	chartDir, err := h.Pull(chart)
	if err != nil {
		return nil, err
	}
	// TODO do not depend on yq
	nginx, err := Command("yq", ".global.images.nginx.tag", chartDir+"/values.yaml")
	if err != nil {
		return nil, err
	}
	alb, err := Command("yq", ".global.images.alb2.tag", chartDir+"/values.yaml")
	if err != nil {
		return nil, err
	}
	return []string{
		registry + "/acp/alb-nginx:" + strings.TrimFunc(nginx, IsNotAlphabetic),
		registry + "/acp/alb2:" + strings.TrimFunc(alb, IsNotAlphabetic),
	}, nil
}

func IsNotAlphabetic(r rune) bool {
	return !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '.')
}
