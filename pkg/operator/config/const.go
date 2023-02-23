package config

type valuePath struct {
	Value interface{}
	Path  []string
}

var BindNic = valuePath{
	Value: "{}",
	Path:  []string{"bindNIC"},
}

var DEFAULT_ENV = map[string]string{
	"ALB_IMAGE":         "alb.img",
	"NGINX_IMAGE":       "nginx.img",
	"LABEL_BASE_DOMAIN": "cpaas.io",
}
