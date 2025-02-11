package ingressnginx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	. "alauda.io/alb2/test/kind/pkg/helper"
	. "alauda.io/alb2/utils"
	"alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	mapset "github.com/deckarep/golang-set/v2"
	gr "github.com/go-resty/resty/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"github.com/xorcare/pointer"
	corev1 "k8s.io/api/core/v1"

	nv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// 期望是无缝兼容的。。用户不需要额外配置，原本通过ingress-nginx 设置auth annotation 能正常使用的，换成alb 仍然能正常使用
// The expectation is seamless compatibility. Users should not need any additional configuration; if the auth annotation set through ingress-nginx works properly, it should still work without issues when switching to ALB.
var _ = Describe("Ingress Auth Test", Ordered, func() {
	l := log.InitKlogV2(log.LogCfg{}).WithName("auth-test")
	ctx := context.Background()
	_ = ctx
	var k *Kubectl
	var kc *K8sClient
	var cfg *rest.Config
	var alb_ip string = ""
	alb_http_port := "11180"
	alb_ip_port := ""
	ingng_ip := ""
	ingng_ip_port := ""
	echo_ip := ""
	echo_public_ip := ""
	BeforeAll(func() {
		// 部署alb，ingress-nginx,echo-resty
		l.Info("fetch info from k8s")
		cfg_, err := RESTFromKubeConfigFile(os.Getenv("KUBECONFIG"))
		Expect(err).Should(BeNil())
		cfg = cfg_

		k = NewKubectl("", cfg, l)
		kc = NewK8sClient(ctx, cfg)

		alb_ips, err := kc.GetPodIp("cpaas-system", "service_name=alb2-auth")
		GinkgoNoErr(err)
		alb_ip = alb_ips[0]
		alb_ip_port = alb_ip + ":" + alb_http_port

		ingng_ips, err := kc.GetPodIp("ingress-nginx", "app.kubernetes.io/name=ingress-nginx")
		GinkgoNoErr(err)
		ingng_ip = ingng_ips[0]
		ingng_ip_port = ingng_ip + ":" + "80"

		_ = k
		_ = alb_http_port
		_ = alb_ip_port
		_ = ingng_ip
		_ = ingng_ip_port
	})

	// 正常情况下，只有auth 成功时，才会在response header中额外加上 auth response设置的cookie
	// Under normal circumstances, the cookie specified in the auth response will only be added to the response header if the authentication is successful.
	// The "always set cookie" option means that the cookie specified in the auth response will be added to the response header regardless of whether the authentication is successful or not.
	// always set cookie 的意思是，无论auth是否成功，都会在response header中额外加上 auth response设置的cookie
	// {{ if $externalAuth.AlwaysSetCookie }}
	// add_header          Set-Cookie $auth_cookie always;
	// {{ else }}
	// add_header          Set-Cookie $auth_cookie;
	// {{ end }}
	DescribeTableSubtree("auth cookie", func(always_set_cookie bool, auth_set_cookie string, upstream_error bool, upstream_set_cookie string, expect_cookie string) {
		ingress_template := `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: auth-check-cookies
  namespace: default
spec:
  rules:
  - host: "auth-check-cookies"
    http:
      paths:
      - backend:
          service:
            name: auth-server
            port:
              number: 80
        path: /
        pathType: Prefix
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: auth-check-cookies-error
  namespace: default
spec:
  rules:
  - host: "auth-check-cookies"
    http:
      paths:
      - backend:
          service:
            name: auth-server
            port:
              number: 80
        path: /error
        pathType: Prefix
`
		ingress := ""
		BeforeAll(func() {
			auth_and_upstream_raw := `
            access_log  /dev/stdout  ;
            error_log   /dev/stdout  info;
            location ~ ^/cookies/set/(?<key>.*)/(?<value>.*) {
                content_by_lua_block {
                    ngx.log(ngx.INFO,"im auth xx "..ngx.var.key)
                    if ngx.var.key ~= "not-set" then
                        ngx.log(ngx.INFO,"im auth. set cookie")
                        ngx.header['Set-Cookie'] = {ngx.var.key.."="..ngx.var.value}
                    end
                    if ngx.var.key == "failed" then
                        local code = 403
                        ngx.status = code
                        ngx.exit(code)
                        return
                    end
                    ngx.say("OK")
                }
            }
            location / {
                content_by_lua_block {
                    ngx.log(ngx.INFO,"im app xx ok "..tostring(ngx.var.http_add_cookie))
                    if ngx.var.http_add_cookie ~= nil then
                        ngx.log(ngx.INFO,"add cookie "..ngx.var.http_add_cookie)
                        ngx.header['Set-Cookie'] = {ngx.var.http_add_cookie}
                    end
                    ngx.say("OK")
                }
            }
            location /error {
                content_by_lua_block {
                    ngx.log(ngx.INFO,"im app xx fail "..tostring(ngx.var.http_add_cookie))
                    local h, err = ngx.req.get_headers()
                    if err ~=nil then
                        ngx.log(ngx.INFO,"err: "..tostring(err))
                    end
                    for k, v in pairs(h) do
                        ngx.log(ngx.INFO,"h "..tostring(k).." : "..tostring(v))
                    end
                    if ngx.var.http_add_cookie ~= nil then
                        ngx.log(ngx.INFO,"add cookie "..ngx.var.http_add_cookie)
                        ngx.header['Set-Cookie'] = {ngx.var.http_add_cookie}
                    end
                    ngx.exit(503)
                }
            }
        `
			echo, err := NewEchoResty("", cfg, l).Deploy(EchoCfg{Name: "auth-server", Image: os.Getenv("ALB_IMAGE"), Ip: "v4", Raw: auth_and_upstream_raw, PodPort: "80", PodHostPort: "60080"})
			GinkgoNoErr(err)
			echo_ip_, err := echo.GetIp()
			GinkgoNoErr(err)
			echo_ip = echo_ip_
			l.Info("echo", "echo ip", echo_ip)
			ingress = Template(ingress_template, map[string]interface{}{
				"echo_ip": echo_ip,
			})
		})
		AfterAll(func() {
			k.Kubectl("delete ingress auth-check-cookies")
			k.Kubectl("delete ingress auth-check-cookies-error")
		})
		do_test := func(ip_port string) {
			if auth_set_cookie != "" {
				key := strings.Split(auth_set_cookie, "=")[0]
				val := strings.Split(auth_set_cookie, "=")[1]
				k.AssertKubectl("annotate", "ingresses", "auth-check-cookies", "--overwrite", "nginx.ingress.kubernetes.io/auth-url=http://"+echo_ip+"/cookies/set/"+key+"/"+val)
				k.AssertKubectl("annotate", "ingresses", "auth-check-cookies-error", "--overwrite", "nginx.ingress.kubernetes.io/auth-url=http://"+echo_ip+"/cookies/set/"+key+"/"+val)
			}
			if always_set_cookie {
				k.AssertKubectl("annotate", "ingresses", "auth-check-cookies", "--overwrite", "nginx.ingress.kubernetes.io/auth-always-set-cookie=true")
				k.AssertKubectl("annotate", "ingresses", "auth-check-cookies-error", "--overwrite", "nginx.ingress.kubernetes.io/auth-always-set-cookie=true")
			} else {
				k.AssertKubectl("annotate", "ingresses", "auth-check-cookies", "--overwrite", "nginx.ingress.kubernetes.io/auth-always-set-cookie=false")
				k.AssertKubectl("annotate", "ingresses", "auth-check-cookies-error", "--overwrite", "nginx.ingress.kubernetes.io/auth-always-set-cookie=false")
			}
			l.Info("sleep ")
			time.Sleep(time.Second * 10)

			url := "http://" + ip_port
			if upstream_error {
				url += "/error"
			}
			l.Info("url", url)
			r := gr.New().R()
			if upstream_set_cookie != "" {
				r.SetHeader("add-cookie", upstream_set_cookie)
			}

			r.SetHeader("HOST", "auth-check-cookies")
			res, err := r.Get(url)
			GinkgoNoErr(err)
			expect_code := 200
			if upstream_error {
				expect_code = 503
			}
			if strings.Contains(auth_set_cookie, "failed") {
				expect_code = 500
			}
			// Expect(res.StatusCode()).Should(Equal(expect_code))
			_ = expect_code
			l.Info("res", "cookie", res.Cookies(), "expect", expect_cookie)
			if expect_cookie == "" {
				Expect(len(res.Cookies())).Should(Equal(0))
			} else {
				expect_cookies := strings.Split(expect_cookie, ",")
				expect_cookie_set := mapset.NewSet(expect_cookies...)
				real_cookie_set := mapset.NewSet(lo.Map(res.Cookies(), func(item *http.Cookie, _ int) string { return item.Raw })...)
				Expect(real_cookie_set.Equal(expect_cookie_set)).Should(BeTrue())
			}
		}

		It("alb should ok", Label("alb", "auth-cookie"), func() {
			ingress, err := YqDo(ingress, `yq ".spec.ingressClassName=\"auth\""`)
			GinkgoNoErr(err)
			l.Info("update ingress", "ingress", ingress, "ip", echo_ip)
			k.KubectlApply(ingress)
			do_test(alb_ip_port)
		})

		It("ingng should ok", Label("ingng", "auth-cookie"), func() {
			ingress, err := YqDo(ingress, `yq ".spec.ingressClassName=\"nginx\""`)
			GinkgoNoErr(err)
			l.Info("update ingress", "ingress", ingress, "ip", echo_ip)
			k.KubectlApply(ingress)
			do_test(ingng_ip_port)
		})
	},
		Entry("always_set_cookie false | upstream fail   | both cookie", false, "auth=auth", true, "up=up", "up=up"),
		Entry("always_set_cookie false | upstream success | both cookie", false, "auth=auth", false, "up=up", "auth=auth,up=up"),
		Entry("always_set_cookie false | upstream success | both cookie", false, "same=auth", false, "same=up", "same=auth,same=up"),
		Entry("always_set_cookie true  | upstream fail    | both cookie", true, "auth=auth", true, "up=up", "auth=auth,up=up"),
		Entry("always_set_cookie true  | upstream success | both cookie", true, "auth=auth", false, "up=up", "auth=auth,up=up"),

		Entry("always_set_cookie true  | auth fail with cookie", true, "failed=xx", false, "", "failed=xx"),
		Entry("always_set_cookie false | auth fail with cookie", false, "failed=xx", false, "", ""),
	)

	// 默认会将 所有的header都发送的auth server
	// 可以通过 proxy_set_header 来指定额外或者要覆盖的header
	// 默认的额外要发送的header是
	// X-Original-URI          $request_uri;
	// X-Scheme                $pass_access_scheme;
	// X-Original-URL          $scheme://$http_host$request_uri;
	// X-Original-Method       $request_method;
	// X-Sent-From             "alb";
	// X-Real-IP               $remote_addr;
	// X-Forwarded-For         $proxy_add_x_forwarded_for;
	// X-Auth-Request-Redirect $request_uri;

	type AuthTestCase struct {
		Title                string
		Annotations          map[string]string
		Cm                   map[string]map[string]string
		ReqHeader            map[string]string
		ExpectAuthReqHeader  map[string]string
		AuthExit             int
		AuthResponseHeader   map[string]string
		AuthResponseBody     string
		ExpectAppReqHeader   map[string]string
		AppExit              int
		AppResponseHeader    map[string]string
		AppResponseBody      string
		ExpectExit           int
		ExpectResponseHeader map[string][]string
		ExpectBody           string
		extra_check          func(g Gomega, state map[string]interface{})
		alb_external_check   func(g Gomega, state map[string]interface{})
		ingng_external_check func(g Gomega, state map[string]interface{})
	}

	init_auth_ingres := func(name string) string {
		ingress_template := `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{.name}}
  namespace: default
  annotations:
     "nginx.ingress.kubernetes.io/auth-url":    "http://{{.echo_ip}}/auth"
spec:
  rules:
  - host: {{.name}}
    http:
      paths:
      - backend:
          service:
            name: auth-server
            port:
              number: 80
        path: /
        pathType: Prefix
`
		return Template(ingress_template, map[string]interface{}{
			"echo_ip": echo_ip,
			"name":    name,
		})
	}

	signin_entries := []TableEntry{
		Entry(func(c AuthTestCase) string { return c.Title }, AuthTestCase{
			Title:       "auth exit with not 401 403 fail should 500",
			Annotations: map[string]string{},
			AuthExit:    404,
			AppExit:     200,
			ExpectExit:  500,
		}),
		Entry(func(c AuthTestCase) string { return c.Title }, AuthTestCase{
			Title:       "auth 401 should 401",
			Annotations: map[string]string{},
			AuthResponseHeader: map[string]string{
				"www-authenticate": "xxx",
				"xx":               "asf",
			},
			ExpectResponseHeader: map[string][]string{
				"Www-Authenticate": {"xxx"},
			},
			AuthExit:   401,
			AppExit:    200,
			ExpectExit: 401,
		}),
		Entry(func(c AuthTestCase) string { return c.Title }, AuthTestCase{
			Title: "auth 401 with signin should 302",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/auth-signin": "http://$host/auth/start?rd=$escaped_request_uri&xx=bb",
			},
			AuthExit:   401,
			AppExit:    200,
			ExpectExit: 302,
			ExpectResponseHeader: map[string][]string{
				"Location": {"http://auth-host/auth/start?rd=%2Fabc\u0026xx=bb"}, // 就是这样的。。\u0026是&
			},
		}),
		Entry(func(c AuthTestCase) string { return c.Title }, AuthTestCase{
			Title: "auth 401 with signin should 302 without rd",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/auth-signin": "http://$host/auth/start?xx=bb",
			},
			AuthExit:   401,
			AppExit:    200,
			ExpectExit: 302,
			ExpectResponseHeader: map[string][]string{
				"Location": {"http://auth-host/auth/start?xx=bb\u0026rd=http://auth-host%2Fabc"}, // 就是这样的。。\u0026是&
			},
		}),

		Entry(func(c AuthTestCase) string { return c.Title }, AuthTestCase{
			Title:       "auth 302",
			Annotations: map[string]string{},
			AuthExit:    302,
			AuthResponseHeader: map[string]string{
				"location": "http://a.com",
			},
			AppExit:    200,
			ExpectExit: 500,
		}),

		Entry(func(c AuthTestCase) string { return c.Title }, AuthTestCase{
			Title:       "auth 403 should 403",
			Annotations: map[string]string{},
			AuthExit:    403,
			AppExit:     200,
			ExpectExit:  403,
		}),

		Entry(func(c AuthTestCase) string { return c.Title }, AuthTestCase{
			Title:       "normal ok",
			Annotations: map[string]string{},
			ReqHeader: map[string]string{
				"authentication": "xxx",
			},
			ExpectAuthReqHeader: map[string]string{
				"authentication":          "xxx",
				"x-original-method":       "GET",
				"x-original-url":          "http://auth-host/abc",
				"x-auth-request-redirect": "/abc",
			},
			AuthExit:           200,
			AuthResponseHeader: map[string]string{},
			AuthResponseBody:   "ok",
			AppExit:            200,
			ExpectAppReqHeader: map[string]string{
				"host":             "auth-host",
				"authentication":   "xxx",
				"x-forwarded-host": "auth-host",
			},
			ExpectExit: 200,
			alb_external_check: func(g Gomega, state map[string]interface{}) {
				header := state["/auth"].((map[string]interface{}))
				g.Expect(header["x-sent-from"].(string) == "alb")
			},
			ingng_external_check: func(g Gomega, state map[string]interface{}) {
				header := state["/auth"].((map[string]interface{}))
				g.Expect(header["x-sent-from"]).ShouldNot(BeNil())
				g.Expect(header["x-sent-from"].(string) == "nginx-ingress-controller")
			},
		}),
	}
	do_auth_test := func(ing_name string, testcase AuthTestCase, ip_port string, extra_check func(Gomega, map[string]interface{})) {
		for ak, av := range testcase.Annotations {
			k.AssertKubectl("annotate", "ingresses", ing_name, "--overwrite", ak+"="+av)
		}
		if testcase.Cm != nil {
			for name, cm_map := range testcase.Cm {
				ns, name := strings.Split(name, "/")[0], strings.Split(name, "/")[1]
				k.Kubectl("delete cm -n " + ns + " " + name)
				cm := corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: ns,
					},
					Data: cm_map,
				}
				_, err := kc.GetK8sClient().CoreV1().ConfigMaps(ns).Create(ctx, &cm, metav1.CreateOptions{})
				GinkgoNoErr(err)
			}
		}
		// 嗯。。。。
		time.Sleep(time.Second * 6)
		real_test := func(g Gomega) {
			id := fmt.Sprintf("%v", time.Now().UnixNano())
			// tell auth server what should do
			expect_auth_behavior := map[string]interface{}{}
			auth_exit := 200
			if testcase.AuthExit != 0 {
				auth_exit = testcase.AuthExit
			}
			app_exit := 200
			if testcase.AppExit != 0 {
				app_exit = testcase.AppExit
			}
			auth_response_header := make(map[string]string)
			if testcase.AuthResponseHeader != nil {
				auth_response_header = testcase.AuthResponseHeader
			}
			auth_response_body := "ok"
			if testcase.AuthResponseBody != "" {
				auth_response_body = testcase.AuthResponseBody
			}
			app_response_header := make(map[string]string)
			if testcase.AppResponseHeader != nil {
				app_response_header = testcase.AppResponseHeader
			}
			app_response_body := "ok"
			if testcase.AppResponseBody != "" {
				app_response_body = testcase.AppResponseBody
			}

			expect_auth_behavior["auth_exit"] = auth_exit
			expect_auth_behavior["auth_response_header"] = auth_response_header
			expect_auth_behavior["auth_response_body"] = auth_response_body
			expect_auth_behavior["app_exit"] = app_exit
			expect_auth_behavior["app_response_header"] = app_response_header
			expect_auth_behavior["app_response_body"] = app_response_body
			_, err := gr.New().R().
				SetHeader("id", id).
				SetBody(expect_auth_behavior).
				Put("http://" + echo_public_ip + ":60080" + "/state")
			g.Expect(err).ShouldNot(HaveOccurred())

			// client send request
			cli := gr.New()
			cli.SetRedirectPolicy(gr.NoRedirectPolicy())
			r := cli.R()
			r.SetHeader("Accept-Encoding", "*")
			r.SetHeader("host", ing_name)
			r.SetHeader("id", id)
			for k, v := range testcase.ReqHeader {
				r.SetHeader(k, v)
			}
			res, _ := r.Get("http://" + ip_port + "/abc")
			l.Info("ret", "code", res.StatusCode(), "header", res.Header(), "body", res.String())
			if testcase.ExpectExit != 0 {
				g.Expect(res.StatusCode()).Should(Equal(testcase.ExpectExit))
			}
			if testcase.ExpectResponseHeader != nil {
				same, patch, err := JsonBelongsTO(res.Header(), testcase.ExpectResponseHeader)
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(same).Should(BeTrue(), "auth req header not match\n"+PrettyJson(patch))
			}

			// so what auth server received?
			res, err = gr.New().R().
				SetHeader("id", id).
				ForceContentType("application/json").
				Get("http://" + echo_public_ip + ":60080" + "/state")
			_ = err
			l.Info("ret", "code", res.StatusCode(), "body", res.String())

			data := map[string]interface{}{}
			err = json.Unmarshal([]byte(res.String()), &data)
			g.Expect(err).ShouldNot(HaveOccurred())
			l.Info("auth all", "data", PrettyJson(data))
			if testcase.ExpectAuthReqHeader != nil {
				l.Info("auth_req", "auth_req_header", PrettyJson(data["/auth"]), "expect_auth_req_header", PrettyJson(testcase.ExpectAuthReqHeader))
				same, patch, err := JsonBelongsTO(data["/auth"], testcase.ExpectAuthReqHeader)
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(same).Should(BeTrue(), "auth req header not match\n"+PrettyJson(patch))
			}

			if data["/"] != nil && len(testcase.ExpectAppReqHeader) > 0 {
				l.Info("app_req", "app_req_header", PrettyJson(data["/auth"]), "expect_app_req_header", PrettyJson(testcase.ExpectAppReqHeader))
				same, patch, err := JsonBelongsTO(data["/"], testcase.ExpectAppReqHeader)
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(same).Should(BeTrue(), "app req header not match\n"+PrettyJson(patch))
			}
			if testcase.extra_check != nil {
				testcase.extra_check(g, data)
			}
			if extra_check != nil {
				extra_check(g, data)
			}
		}
		real_test(NewGomegaWithT(GinkgoT()))
	}
	DescribeTableSubtree("auth signin", func(testcase AuthTestCase) {
		ingress := ""
		BeforeAll(func() {
			auth_resty, err := NewAuthResty(l, cfg)
			GinkgoNoErr(err)
			echo_ip, echo_public_ip, err = auth_resty.GetIpAndHostIp()
			GinkgoNoErr(err)
			ingress = init_auth_ingres("auth-host")
		})
		AfterAll(func() {
			k.Kubectl("delete ingress auth-host")
		})
		BeforeEach(func() {
			k.Kubectl("delete ingress auth-host")
		})
		It("ingng should ok", Label("ingng", "auth-signin"), func() {
			ingress, err := YqDo(ingress, `yq ".spec.ingressClassName=\"nginx\""`)
			GinkgoNoErr(err)
			l.Info("update ingress", "ingress", ingress, "ip", echo_ip)
			k.KubectlApply(ingress)
			l.Info("ingng", "host", ingng_ip_port)
			do_auth_test("auth-host", testcase, ingng_ip_port, testcase.ingng_external_check)
		})

		It("alb should ok", Label("alb", "auth-signin"), func() {
			ingress, err := YqDo(ingress, `yq ".spec.ingressClassName=\"auth\""`)
			GinkgoNoErr(err)
			k.Kubectl("delete ingress auth-signin")
			l.Info("update ingress", "ingress", ingress, "ip", echo_ip)
			k.KubectlApply(ingress)
			do_auth_test("auth-host", testcase, alb_ip_port, testcase.alb_external_check)
		})
	}, signin_entries)

	// nginx.ingress.kubernetes.io/auth-response-headers: <Response_Header_1, ..., Response_Header_n> to specify headers to pass to backend once authentication request completes.
	// nginx.ingress.kubernetes.io/auth-proxy-set-headers: <ConfigMap> the name of a ConfigMap that specifies headers to pass to the authentication service
	// nginx.ingress.kubernetes.io/auth-request-redirect: <Request_Redirect_URL> to specify the X-Auth-Request-Redirect header value.
	extra_headers_entries := []TableEntry{
		Entry(func(c AuthTestCase) string { return c.Title }, AuthTestCase{
			Title: "auth-response-headers",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/auth-response-headers": "X-Auth-Request-Redirect,X-Test-Auth",
			},
			ReqHeader: map[string]string{
				"a": "b",
			},
			AuthResponseHeader: map[string]string{
				"x-test-auth": "xxx",
			},
			ExpectAppReqHeader: map[string]string{
				"x-test-auth": "xxx",
				"a":           "b",
			},
		}),

		Entry(func(c AuthTestCase) string { return c.Title }, AuthTestCase{
			Title: "invalid auth-proxy-set-headers",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/auth-proxy-set-headers": "default/xx-not-exist",
			},
			ExpectExit: 503,
		}),
		Entry(func(c AuthTestCase) string { return c.Title }, AuthTestCase{
			Title: "auth-proxy-set-headers",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/auth-proxy-set-headers": "default/custom-header",
			},
			Cm: map[string]map[string]string{
				"default/custom-header": {
					"X-Different-Name":         "true",
					"X-Request-Start":          "t=${msec}",
					"xx-host-x":                "$http_host",
					"Complex":                  "http://${http_host} ?!(){}[]@<>=-+*#&|~^% ",
					"X-Using-Nginx-Controller": "true",
				},
			},
			ExpectAuthReqHeader: map[string]string{
				"x-different-name":         "true",
				"xx-host-x":                "auth-extra",
				"complex":                  "http://auth-extra ?!(){}[]@\u003c\u003e=-+*#\u0026|~^%",
				"x-using-nginx-controller": "true",
			},
			extra_check: func(g Gomega, data map[string]interface{}) {
				start := data["/auth"].(map[string]interface{})["x-request-start"].(string)
				l.Info("start", "start", start)
				g.Expect(strings.HasPrefix(start, "t=173")).Should(BeTrue())
			},
		}),
		Entry(func(c AuthTestCase) string { return c.Title }, AuthTestCase{
			Title: "auth-request-redirect",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/auth-request-redirect": "https://a.b.c",
			},
			ExpectAuthReqHeader: map[string]string{
				"x-auth-request-redirect": "https://a.b.c",
			},
		}),
	}

	DescribeTableSubtree("auth extra", func(testcase AuthTestCase) {
		ingress := ""
		BeforeAll(func() {
			auth_resty, err := NewAuthResty(l, cfg)
			GinkgoNoErr(err)
			echo_ip, echo_public_ip, err = auth_resty.GetIpAndHostIp()
			GinkgoNoErr(err)
			ingress = init_auth_ingres("auth-extra")
		})

		AfterAll(func() {
			k.Kubectl("delete ingress auth-extra")
		})
		BeforeEach(func() {
			k.Kubectl("delete ingress auth-extra")
		})

		It("ingng should ok", Label("ingng", "auth-extra"), func() {
			ingress, err := YqDo(ingress, `yq ".spec.ingressClassName=\"nginx\""`)
			GinkgoNoErr(err)
			l.Info("update ingress", "ingress", ingress, "ip", echo_ip)
			k.KubectlApply(ingress)
			l.Info("ingng", "host", ingng_ip_port)
			do_auth_test("auth-extra", testcase, ingng_ip_port, testcase.ingng_external_check)
		})

		It("alb should ok", Label("alb", "auth-extra"), func() {
			ingress, err := YqDo(ingress, `yq ".spec.ingressClassName=\"auth\""`)
			GinkgoNoErr(err)
			k.Kubectl("delete ingress auth-extra")
			l.Info("update ingress", "ingress", ingress, "ip", echo_ip)
			k.KubectlApply(ingress)
			do_auth_test("auth-extra", testcase, alb_ip_port, testcase.alb_external_check)
		})
	}, extra_headers_entries)

	type IngCfg struct {
		title      string
		annotation map[string]string
		secret     *corev1.Secret
	}

	type AuthBasicTestCase struct {
		IngCfg
		ReqHeader       map[string]string
		ExpectResHeader map[string]string
		ExpectCode      int
	}
	auth_basic_entries := []TableEntry{
		Entry(func(c AuthBasicTestCase) string { return c.title }, AuthBasicTestCase{
			IngCfg: IngCfg{
				title: "apr1 file type should ok 401",
				annotation: map[string]string{
					"nginx.ingress.kubernetes.io/auth-realm":       "default",
					"nginx.ingress.kubernetes.io/auth-secret":      "default/auth-secret",
					"nginx.ingress.kubernetes.io/auth-secret-type": "auth-file",
					"nginx.ingress.kubernetes.io/auth-type":        "basic",
				},
				secret: &corev1.Secret{
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						// foo : bar
						"auth": []byte("foo:$apr1$qICNZ61Q$2iooiJVUAMmprq258/ChP1"), //  cspell:disable-line
					},
				},
			},
			ExpectCode: 401,
			ExpectResHeader: map[string]string{
				"Www-Authenticate": "Basic realm=\"default\"",
			},
		}),
		Entry(func(c AuthBasicTestCase) string { return c.title }, AuthBasicTestCase{
			IngCfg: IngCfg{
				title: "apr1 file type should ok 200",
				annotation: map[string]string{
					"nginx.ingress.kubernetes.io/auth-realm":       "default",
					"nginx.ingress.kubernetes.io/auth-secret":      "default/auth-secret",
					"nginx.ingress.kubernetes.io/auth-secret-type": "auth-file",
					"nginx.ingress.kubernetes.io/auth-type":        "basic",
				},
				secret: &corev1.Secret{
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						// foo : bar
						"auth": []byte("foo:$apr1$qICNZ61Q$2iooiJVUAMmprq258/ChP1"), //  cspell:disable-line
					},
				},
			},
			ExpectCode: 200,
			ReqHeader: map[string]string{
				"Authorization": "Basic Zm9vOmJhcg==", //  cspell:disable-line
			},
			ExpectResHeader: map[string]string{},
		}),
		Entry(func(c AuthBasicTestCase) string { return c.title }, AuthBasicTestCase{
			IngCfg: IngCfg{
				title: "apr1 map type should ok 200",
				annotation: map[string]string{
					"nginx.ingress.kubernetes.io/auth-realm":       "default",
					"nginx.ingress.kubernetes.io/auth-secret":      "default/auth-secret",
					"nginx.ingress.kubernetes.io/auth-secret-type": "auth-map",
					"nginx.ingress.kubernetes.io/auth-type":        "basic",
				},
				secret: &corev1.Secret{
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"foo": []byte("$apr1$qICNZ61Q$2iooiJVUAMmprq258/ChP1"), //  cspell:disable-line
					},
				},
			},
			ExpectCode: 200,
			ReqHeader: map[string]string{
				"Authorization": "Basic Zm9vOmJhcg==", //  cspell:disable-line
			},
			ExpectResHeader: map[string]string{},
		}),
	}
	DescribeTableSubtree("auth basic", func(testcase AuthBasicTestCase) {
		init_auth_basic_ingres := func(class string, kt *Kubectl) {
			cfg := testcase.IngCfg
			annotations := cfg.annotation
			p_type := nv1.PathTypeExact
			ing := nv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "auth-basic",
					Namespace:   "default",
					Annotations: annotations,
				},
				Spec: nv1.IngressSpec{
					IngressClassName: pointer.String(class),
					Rules: []nv1.IngressRule{
						{
							Host: "auth-basic",
							IngressRuleValue: nv1.IngressRuleValue{
								HTTP: &nv1.HTTPIngressRuleValue{
									Paths: []nv1.HTTPIngressPath{
										{
											Path:     "/ok",
											PathType: &p_type,
											Backend: nv1.IngressBackend{
												Service: &nv1.IngressServiceBackend{
													Name: "auth-server",
													Port: nv1.ServiceBackendPort{
														Number: 80,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}
			kt.Kubectl("delete ingress -n default auth-basic")
			kt.Kubectl("delete secret -n default auth-secret")
			time.Sleep(3 * time.Second)
			if testcase.secret != nil {
				cfg.secret.ObjectMeta = metav1.ObjectMeta{
					Name:      "auth-secret",
					Namespace: "default",
				}
				_, err := kc.GetK8sClient().CoreV1().Secrets("default").Create(ctx, testcase.secret, metav1.CreateOptions{})
				GinkgoNoErr(err)
			}
			_, err := kc.GetK8sClient().NetworkingV1().Ingresses("default").Create(ctx, &ing, metav1.CreateOptions{})
			GinkgoNoErr(err)
			time.Sleep(5 * time.Second)
		}

		BeforeAll(func() {
			auth_resty, err := NewAuthResty(l, cfg)
			GinkgoNoErr(err)
			echo_ip, echo_public_ip, err = auth_resty.GetIpAndHostIp()
			GinkgoNoErr(err)
		})

		AfterAll(func() {
			k.Kubectl("delete ingress auth-basic")
		})
		BeforeEach(func() {
			k.Kubectl("delete ingress auth-basic")
		})
		do_auth_basic_test := func(base_url string) {
			url := "http://" + base_url + "/ok"
			l.Info("curl", "url", url)
			r := gr.New().R().
				SetHeader("HOST", "auth-basic").
				SetHeaders(testcase.ReqHeader)
			res, err := r.Get(url)
			GinkgoNoErr(err)
			l.Info("res", "body", res.Body(), "status", res.Status(), "header", res.Header())
			GinkgoAssertTrue(res.StatusCode() == testcase.ExpectCode, "")
			for k, v := range testcase.ExpectResHeader {
				GinkgoAssertStringEq(strings.Join(res.Header()[k], ","), v, "")
			}
		}
		It("ingng should ok", Label("ingng", "auth-basic"), func() {
			init_auth_basic_ingres("nginx", k)
			do_auth_basic_test(ingng_ip_port)
		})

		It("alb should ok", Label("alb", "auth-basic"), func() {
			init_auth_basic_ingres("auth", k)
			do_auth_basic_test(alb_ip_port)
		})
	}, auth_basic_entries)
})
