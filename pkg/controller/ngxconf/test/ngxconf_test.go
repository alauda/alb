package ngxconf

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lithammer/dedent"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gngc "github.com/tufanbarisyildirim/gonginx/config"
	gngd "github.com/tufanbarisyildirim/gonginx/dumper"
	gngp "github.com/tufanbarisyildirim/gonginx/parser"

	. "alauda.io/alb2/pkg/controller/ngxconf"
	. "alauda.io/alb2/pkg/controller/ngxconf/types"
	. "alauda.io/alb2/pkg/utils/test_utils"
	. "alauda.io/alb2/utils/test_utils"
)

var (
	_ = fmt.Println
	l = GinkgoLog()
)

func TestNgxc(t *testing.T) {
	t.Logf("ok")
	RegisterFailHandler(Fail)
	RunSpecs(t, "nginx config")
}

var _ = Describe("common", func() {
	It("test render nginx ", func() {
		config := NginxTemplateConfig{
			Frontends: map[string]FtConfig{
				"8081-http": {
					Port:            8081,
					Protocol:        "http",
					IpV4BindAddress: []string{"192.168.0.1", "192.168.0.3"},
					IpV6BindAddress: []string{"[::1]", "[::2]"},
				},
			},
			NginxParam: NginxParam{EnableIPV6: true},
			Flags:      DefaulNgxTmplFlags(),
		}
		configStr, err := RenderNginxConfigEmbed(config)
		GinkgoNoErr(err)
		l.Info("ngxconf", "cf", configStr)
		GinkgoAssertTrue(strings.Contains(configStr, "listen    192.168.0.1:8081"), "")
		GinkgoAssertTrue(strings.Contains(configStr, "listen    192.168.0.3:8081"), "")
		GinkgoAssertTrue(strings.Contains(configStr, "listen    [::1]:8081"), "")
		GinkgoAssertTrue(strings.Contains(configStr, "listen    [::2]:8081"), "")
	})
})

var demo = NginxTemplateConfig{
	Name:      "mock-alb",
	NginxBase: "/alb/nginx",
	RestyBase: "/usr/local/openresty",
	ShareBase: "/etc/alb2/nginx",
	TweakBase: "/alb/tweak",
	Frontends: map[string]FtConfig{
		"8081-http": {
			Port:     8081,
			Protocol: "http",
		},
	},
	Flags:      DefaulNgxTmplFlags(),
	NginxParam: NginxParam{EnableIPV6: true},
}

var _ = DescribeTable("ngx config should work",
	func(configf func() NginxTemplateConfig, massert func(p *gngc.Config, configStr string)) {
		config := configf()
		l.Info("test", "config", fmt.Sprintf("%+v", config))
		configStr, err := RenderNginxConfigEmbed(config)
		Expect(err).To(BeNil())
		p, err := gngp.NewStringParser(configStr, gngp.WithSkipValidDirectivesErr()).Parse()
		fmt_str := gngd.DumpConfig(p, gngd.IndentedStyle)
		Expect(err).To(BeNil())
		massert(p, fmt_str)
	},
	Entry(
		"default tweak base and nginx base",
		func() NginxTemplateConfig {
			return demo
		},
		func(p *gngc.Config, ngxraw string) {
			{
				ds := FindNestDirectives(p, "http", "upstream.include[0]").(*gngc.Include).IncludePath
				Expect(ds).To(Equal("/alb/tweak/upstream.conf"))
			}
			{
				ds := FindNestDirectives(p, "stream", "init_worker_by_lua_file[0]").GetParameters()[0]
				Expect(ds).To(Equal("/alb/nginx/lua/phase/init_worker_phase.lua"))
			}
		},
	),
	Entry(
		"custom tweak base and nginx base",
		func() NginxTemplateConfig {
			x := demo
			x.TweakBase = "/xxx/tweak"
			x.NginxBase = "/xxx/nginx"
			return x
		},
		func(p *gngc.Config, ngxraw string) {
			{
				ds := FindNestDirectives(p, "http", "upstream.include[0]").(*gngc.Include).IncludePath
				Expect(ds).To(Equal("/xxx/tweak/upstream.conf"))
			}
			{
				ds := FindNestDirectives(p, "stream", "init_worker_by_lua_file[0]").GetParameters()[0]
				Expect(ds).To(Equal("/xxx/nginx/lua/phase/init_worker_phase.lua"))
			}
		}),
	Entry(
		"custom location should wrapper with our cfg",
		func() NginxTemplateConfig {
			x := demo
			ft := x.Frontends["8081-http"]
			ft.Protocol = "http"
			ft.Port = 8081
			ft.IpV4BindAddress = []string{"0.0.0.1"}
			ft.CustomLocation = []FtCustomLocation{
				{
					Name: "modsecurity_p1",
					LocationRaw: dedent.Dedent(`
						modsecurity on;
						modsecurity_rules '
							SecRuleEngine On
						';
					`),
				},
			}
			x.Frontends["8081-http"] = ft
			return x
		},
		func(p *gngc.Config, ngxraw string) {
			loc := FindNestDirectives(p, "http", "server[0].location[0]").(*gngc.Location)
			Expect(loc.Match).To(Equal("@modsecurity_p1"))
			blk := loc.GetBlock()
			l.Info("raw", "\nraw\n", ngxraw, "\nloc\n", loc.Match, "\ncur-blk\n", gngd.DumpBlock(blk, gngd.IndentedStyle))
			AssertNgxBlockEq(blk, dedent.Dedent(`
				internal;
				modsecurity on;
				modsecurity_rules '
					SecRuleEngine On
				';
				set $location_mode sub;
				rewrite_by_lua_file /alb/nginx/lua/phase/l7_rewrite_phase.lua;
				proxy_pass $backend_protocol://http_backend;
				header_filter_by_lua_file /alb/nginx/lua/l7_header_filter.lua;
			`))
		},
	),

	Entry(
		"pick http ctx only",
		func() NginxTemplateConfig {
			x := demo
			x.Flags.ShowEnv = false
			x.Flags.ShowRoot = false
			x.Flags.ShowHttp = true
			x.Flags.ShowStream = false
			return x
		},
		func(p *gngc.Config, ngxraw string) {
			l.Info("http", "\nhttp\n", ngxraw)
		},
	),
	Entry(
		"pick root env only",
		func() NginxTemplateConfig {
			x := demo
			x.Flags.ShowEnv = true
			x.Flags.ShowRoot = false
			x.Flags.ShowHttp = false
			x.Flags.ShowStream = false
			return x
		},
		func(p *gngc.Config, ngxraw string) {
			l.Info("raw", "\nraw\n", ngxraw)
		},
	),
	Entry(
		"test nginx",
		func() NginxTemplateConfig {
			x := demo
			http_8081 := x.Frontends["8081-http"]
			http_8081.Protocol = "http"
			http_8081.Port = 8081
			http_8081.IpV4BindAddress = []string{"0.0.0.1"}
			x.Frontends["8081-http"] = http_8081

			tcp_81 := FtConfig{
				Protocol:        "tcp",
				Port:            81,
				IpV4BindAddress: []string{"0.0.0.1"},
			}
			x.Frontends["81-tcp"] = tcp_81
			udp_82 := FtConfig{
				Protocol:        "udp",
				Port:            82,
				IpV4BindAddress: []string{"0.0.0.1"},
			}
			x.Frontends["82-udp"] = udp_82
			x.StreamExtra = `
			server {
				listen 9001 udp;
				content_by_lua_block {
					ngx.log(ngx.INFO,"udp socket connect")
					local sock,err = ngx.req.socket()
					local data, err = sock:receive()
					if err ~= nil then
						sock:send("err "..tostring(err))
					end
					sock:send(data)
				}
			}
			`
			return x
		},
		func(p *gngc.Config, ngxraw string) {
			l.Info("raw", "\nraw\n", ngxraw)
			// stream should include the stream-common
			{
				stream_include := FindNestDirectives(p, "stream", "include[0]").(*gngc.Include).IncludePath
				Expect(stream_include).To(Equal("/alb/tweak/stream-common.conf"))
			}
			// first server should be stream extra since we have it.
			{
				stream_9001_lis := FindNestDirectives(p, "stream", "server[0].listen[0]").GetParameters()
				Expect(stream_9001_lis).To(Equal([]string{"9001", "udp"}))
			}
			// tcp 81
			{
				stream_81_lis := FindNestDirectives(p, "stream", "server[1].listen[0]").GetParameters()
				Expect(stream_81_lis).To(Equal([]string{"0.0.0.1:81"}))
			}
			// udp 82
			{
				stream_82_lis := FindNestDirectives(p, "stream", "server[2].listen[0]").GetParameters()
				Expect(stream_82_lis).To(Equal([]string{"0.0.0.1:82", "udp"}))
			}
		},
	),
	Entry(
		"test nginx from yaml",
		func() NginxTemplateConfig {
			cfg, err := NgxTmplCfgFromYaml(`
enableHTTP2: true
flags:
  showHttp: true
frontends:
  https_443:
    port: 443
    enableHTTP2: true
    protocol: https
    ipV4BindAddress:
      - 0.0.0.1
`)
			GinkgoAssert(err, "should not error")
			return *cfg
		},
		func(p *gngc.Config, ngxraw string) {
			l.Info("xraw", "\nraw\n", ngxraw)
		},
	),
)
