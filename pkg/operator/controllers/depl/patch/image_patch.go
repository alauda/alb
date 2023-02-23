package patch

import (
	cfg "alauda.io/alb2/pkg/operator/config"
)

// TODO 决定一个deployment具体要用那个image，和status中imagepatch字段,应该在一个地方来管理
func GenImagePatch(conf *cfg.ALB2Config, operator cfg.OperatorCfg) (hasPatch bool, alb string, nginx string) {
	// 要升级的版本和patch中的版本一致
	hasPatch = false
	alb = operator.AlbImage
	nginx = operator.NginxImage
	for _, p := range conf.Overwrite.Image {
		if p.Target == "" || p.Target == operator.Version {
			if p.Alb != "" {
				hasPatch = true
				alb = p.Alb
			}
			if p.Nginx != "" {
				hasPatch = true
				nginx = p.Nginx
			}
		}
	}
	return hasPatch, alb, nginx
}
