package helper

import (
	// "path/filepath"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	cp "github.com/otiai10/copy"
)

// 了解alb的chart的内容，给定一个alb的chart，可以从中获取信息

type AlbChartExt struct {
	base              string
	helm              *Helm
	log               logr.Logger
	chart             string
	overwrite         string
	OverwriteRegistry string
}

func NewAlbChart() *AlbChartExt {
	return &AlbChartExt{}
}

func (a *AlbChartExt) WithHelm(helm *Helm) *AlbChartExt {
	a.helm = helm
	return a
}

func (a *AlbChartExt) WithLog(l logr.Logger) *AlbChartExt {
	a.log = l
	return a
}

func (a *AlbChartExt) Load(chart string) (*AlbChartExt, error) {
	if a.helm == nil {
		a.helm = NewHelm(a.base, nil, a.log)
	}
	a.log.Info("load chart", "chart", chart)
	if _, err := os.Stat(chart); err == nil {
		a.log.Info("load chart from dir", "chart", chart)
		err := a.LoadFromDir(chart)
		if err != nil {
			return nil, err
		}
		return a, nil
	}
	a.log.Info("load from url", "url", chart)
	err := a.LoadFromUrl(chart)
	return a, err
}

func (a *AlbChartExt) WithBase(base string) *AlbChartExt {
	a.base = BaseWithDir(base, "alb-chart")
	return a
}

func (a *AlbChartExt) WithOverwrite(overwrite string) *AlbChartExt {
	a.overwrite = overwrite
	return a
}

func LoadAlbChartFromUrl(base string, helm *Helm, chart string, log logr.Logger) (*AlbChartExt, error) {
	ac := &AlbChartExt{
		helm: helm,
	}
	ac = ac.WithBase(base).WithHelm(helm)
	ac.log = log
	err := ac.LoadFromUrl(chart)
	if err != nil {
		return nil, err
	}

	return ac, nil
}

func (ac *AlbChartExt) LoadFromUrl(chart string) error {
	ac.log.Info("pull", "chart", chart)
	chartdir, err := ac.helm.Pull(chart)
	if err != nil {
		return err
	}
	ac.chart = chartdir
	return nil
}

func (ac *AlbChartExt) LoadFromDir(chart string) error {
	name := filepath.Base(chart)
	ac.chart = path.Join(ac.base, name)
	err := cp.Copy(chart, ac.chart)
	if err != nil {
		return err
	}
	return nil
}

func LoadAlbChartFromDir(base string, helm *Helm, chart string, log logr.Logger) (*AlbChartExt, error) {
	ac := &AlbChartExt{
		helm: helm,
	}
	ac = ac.WithBase(base).WithHelm(helm)
	ac.log = log
	err := ac.LoadFromDir(chart)
	if err != nil {
		return nil, err
	}
	return ac, nil
}

func (a *AlbChartExt) GetVersion() (string, error) {
	version, err := Command("yq", ".version", a.chart+"/Chart.yaml")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(version), nil
}

func (a *AlbChartExt) GetImage() (albImage string, nginxImage string, err error) {
	chartDir := a.chart
	// TODO do not depend on yq
	registry, err := Command("yq", ".global.registry.address", chartDir+"/values.yaml")
	if err != nil {
		return "", "", err
	}
	nginx, err := Command("yq", ".global.images.nginx.tag", chartDir+"/values.yaml")
	if err != nil {
		return "", "", err
	}
	alb, err := Command("yq", ".global.images.alb2.tag", chartDir+"/values.yaml")
	if err != nil {
		return "", "", err
	}
	registry = strings.TrimFunc(registry, IsNotAlphabetic)
	if a.OverwriteRegistry != "" {
		registry = a.OverwriteRegistry
	}
	return registry + "/acp/alb-nginx:" + strings.TrimFunc(nginx, IsNotAlphabetic), registry + "/acp/alb2:" + strings.TrimFunc(alb, IsNotAlphabetic), nil
}
func (a *AlbChartExt) ListImage() ([]string, error) {
	alb, nginx, err := a.GetImage()
	if err != nil {
		return nil, err
	}
	return []string{alb, nginx}, err
}

func (a *AlbChartExt) GetChartDir() string {
	return a.chart
}

func IsNotAlphabetic(r rune) bool {
	return !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '.')
}
