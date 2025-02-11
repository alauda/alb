package types

import albv1 "alauda.io/alb2/pkg/apis/alauda/v1"

// a config used for nginx.tmpl to generate nginx.conf
type NginxTemplateConfig struct {
	Name            string              `yaml:"name"`
	TweakBase       string              `yaml:"tweakBase"` // /alb/tweak
	NginxBase       string              `yaml:"nginxBase"` // /alb/nginx
	RestyBase       string              `yaml:"restyBase"` // /usr/local/openresty/
	ShareBase       string              `yaml:"shareBase"` // the /etc/alb2/nginx
	Frontends       map[string]FtConfig `yaml:"frontends"`
	Metrics         MetricsConfig       `yaml:"metrics"`
	Resolver        string              `yaml:"resolver"`
	ResolverTimeout string              `yaml:"resolver_timeout"`
	TweakHash       string              `yaml:"tweakHash"`
	Phase           string              `yaml:"phase"`
	Base            string              `yaml:"base"`
	NginxParam      `yaml:",inline"`
	RootExtra       string `yaml:"rootExtra"`
	HttpExtra       string `yaml:"httpExtra"`
	StreamExtra     string `yaml:"streamExtra"`
	Flags           Flags  `yaml:"flags"` // render part of nginx.conf. used in AlaudaLib.pm
}

type FtConfig struct {
	Port            int                `yaml:"port"`
	Protocol        albv1.FtProtocol   `yaml:"protocol"`
	EnableHTTP2     bool               `yaml:"enableHTTP2"`
	CertificateName string             `yaml:"certificateName"`
	IpV4BindAddress []string           `yaml:"ipV4BindAddress"`
	IpV6BindAddress []string           `yaml:"ipV6BindAddress"`
	CustomLocation  []FtCustomLocation `yaml:"customLocation"`
}

// 目前我们直接把nginx 配置merge到一个文件里去
type FtCustomLocation struct {
	Name        string `yaml:"name"`
	LocationRaw string `yaml:"locationRaw"`
}

type MetricsConfig struct {
	Port            int      `yaml:"port"`
	IpV4BindAddress []string `yaml:"ipV4BindAddress"`
	IpV6BindAddress []string `yaml:"ipV6BindAddress"`
}

type NginxParam struct {
	EnablePrometheus bool   `yaml:"enablePrometheus"`
	EnableIPV6       bool   `yaml:"enableIPV6"`
	EnableHTTP2      bool   `yaml:"enableHTTP2"`
	CPUNum           string `yaml:"cpuNum"`
	MetricsPort      int    `yaml:"metricsPort"`
	Backlog          int    `yaml:"backlog"`
	EnableGzip       bool   `yaml:"enableGzip"`
	GzipLevel        int    `yaml:"gzipLevel"`
	GzipMinLength    int    `yaml:"gzipMinLength"`
	GzipTypes        string `yaml:"gzipTypes"`
}

type Flags struct {
	ShowEnv         bool `yaml:"showEnv"`
	ShowRoot        bool `yaml:"showRoot"`      // our default root conf
	ShowRootExtra   bool `yaml:"showRootExtra"` // user define custom nginx config in root
	ShowHttp        bool `yaml:"showHttp"`
	ShowStream      bool `yaml:"showStream"`
	ShowHttpWrapper bool `yaml:"showHttpWrapper"`
	ShowInitWorker  bool `yaml:"showInitWorker"`
	ShowMimeTypes   bool `yaml:"showMimeTypes"`
}

func DefaulNgxTmplFlags() Flags {
	return Flags{
		ShowEnv:         true,
		ShowRoot:        true,
		ShowRootExtra:   true,
		ShowHttp:        true,
		ShowStream:      true,
		ShowHttpWrapper: true,
		ShowInitWorker:  true,
		ShowMimeTypes:   true,
	}
}
