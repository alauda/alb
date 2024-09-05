package ngxconf

import (
	"bytes"
	_ "embed"
	"os"
	"text/template"

	. "alauda.io/alb2/pkg/controller/ngxconf/types"
)

//go:embed nginx.tmpl
var NgxTmplEmbed string

func RenderNgxRaw(config NginxTemplateConfig, tmpl string) (string, error) {
	t, err := template.New("nginx").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, config); err != nil {
		return "", err
	}
	return tpl.String(), nil
}

func RenderNgxFromFile(config NginxTemplateConfig, tmplPath string) (string, error) {
	tmplBytes, err := os.ReadFile(tmplPath)
	if err != nil {
		return "", err
	}
	return RenderNgxRaw(config, string(tmplBytes))
}

func RenderNginxConfigEmbed(config NginxTemplateConfig) (string, error) {
	return RenderNgxRaw(config, NgxTmplEmbed)
}
