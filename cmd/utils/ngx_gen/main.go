package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	. "alauda.io/alb2/pkg/controller/ngxconf"
)

func main() {
	ngx_raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	ngx, err := NgxTmplCfgFromYaml(string(ngx_raw))
	if err != nil {
		panic(err)
	}

	out, err := RenderNginxConfigEmbed(*ngx)
	if err != nil {
		panic(err)
	}
	fmt.Print(strings.TrimSpace(out))
}
