package auth

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	u "alauda.io/alb2/utils"
	"github.com/kr/pretty"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	nv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "alauda.io/alb2/controller/types"
	av1 "alauda.io/alb2/pkg/apis/alauda/v1"
	. "alauda.io/alb2/pkg/controller/ext/auth/types"
	. "alauda.io/alb2/pkg/utils"
	. "alauda.io/alb2/utils/test_utils"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	_ = fmt.Println
	l = ConsoleLog()
)

func TestAuth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "auth config")
}

var _ = Describe("auth", func() {
	t := GinkgoT()

	It("parse var-string", func() {
		xx := []string{"a", "b", "c", "&xx"}
		fmt.Println(xx)
		out, _ := json.Marshal(xx)
		fmt.Println(string(out))
	})

	It("parse var-string", func() {
		cases := []struct {
			name     string
			input    string
			expected VarString
		}{
			{
				name:     "simple string",
				input:    "xx xx xx",
				expected: VarString{"xx xx xx"},
			},
			{
				name:     "string with variable",
				input:    "xx $xx xx",
				expected: VarString{"xx ", "$xx", " xx"},
			},
			{
				name:     "string with variable and -",
				input:    "$x-$xx$s dd xx-dd x",
				expected: VarString{"$x", "-", "$xx", "$s", " dd xx-dd x"},
			},
			{
				name:  "complex var string",
				input: `'[$time_local] $remote_addr "$host" "$request" ' '$status $upstream_status $upstream_addr ' '"$http_user_agent" "$http_x_forwarded_for" ' '$request_time $upstream_response_time $upstream_bytes_received'`,
				expected: VarString{
					"'[", "$time_local", "] ", "$remote_addr", " \"", "$host", "\" \"", "$request", "\" ' '", "$status", " ", "$upstream_status", " ", "$upstream_addr", " ' '\"", "$http_user_agent", "\" \"", "$http_x_forwarded_for", "\" ' '", "$request_time", " ", "$upstream_response_time", " ", "$upstream_bytes_received", "'",
				},
			},
			{
				name:  "url",
				input: `http://$host/auth/start?rd=$escaped_request_uri&xx=bb`,
				expected: VarString{
					"http://", "$host", "/auth/start?rd=", "$escaped_request_uri", "&xx=bb",
				},
			},
			{
				name:  "url",
				input: `http://${host}{x}.-~!@#%^&*()_+|}{':?$es  $$$a $~`,
				expected: VarString{
					"http://", "$host", "{x}.-~!@#%^&*()_+|}{':?", "$es", "  ", "$", "$", "$a", " ", "$", "~",
				},
			},
		}

		for _, tc := range cases {
			vs, err := ParseVarString(tc.input)
			assert.NoError(t, err)
			l.Info("check", "real", vs, "exp", tc.expected)
			assert.Equal(t, tc.expected, vs)
		}
	})

	It("simple reflect perf", func() {
		annotation := map[string]string{
			"xxx/url": "abc",
			"xxx/xx":  "xyz",
		}
		type A struct {
			F1 string `annotation:"url"`
			F2 string `annotation:"xx"`
		}
		simple := func(a *A, annotation map[string]string) {
			if v, ok := annotation["xxx/url"]; ok {
				a.F1 = v
			}
			if v, ok := annotation["xxx/xx"]; ok {
				a.F2 = v
			}
		}
		a := A{}
		show_delay := func(f func()) time.Duration {
			start := time.Now()
			f()
			end := time.Now()
			return end.Sub(start)
		}
		fmt.Println(show_delay(func() {
			for i := 0; i < 100000; i++ {
				simple(&a, annotation)
			}
		}))
		fmt.Println(show_delay(func() {
			for i := 0; i < 100000; i++ {
				ResolverStructFromAnnotation(&a, annotation, ResolveAnnotationOpt{
					Prefix: []string{"xxx"},
				})
			}
		}))
		// 10w 1ms vs 100ms
	})

	It("resolve from annotation should ok", func() {
		type A struct {
			F1 string `annotation:"url"`
			F2 string `annotation:"xx"`
		}
		type AA struct {
			A
			B string `annotation:"b"`
		}

		annotation := map[string]string{
			"xxx/url": "abc",
			"xxx/xx":  "xyz",
			"xxx/b":   "hi",
			"aaa/url": "123",
		}
		a := A{}
		ResolverStructFromAnnotation(&a, annotation, ResolveAnnotationOpt{
			Prefix: []string{"xxx"},
		})
		assert.Equal(t, a, A{F1: "abc", F2: "xyz"})
		ResolverStructFromAnnotation(&a, annotation, ResolveAnnotationOpt{
			Prefix: []string{"aaa", "xxx"},
		})
		assert.Equal(t, a, A{F1: "123", F2: "xyz"})
		// support embed struct
		aa := AA{}
		ResolverStructFromAnnotation(&aa, annotation, ResolveAnnotationOpt{
			Prefix: []string{"aaa", "xxx"},
		})
		fmt.Printf("%+v", aa)
		assert.Equal(t, aa, AA{A: A{F1: "123", F2: "xyz"}, B: "hi"})
	})

	It("resolve ingress annotation should ok ", func() {
		type Case struct {
			title       string
			annotations map[string]string
			rule_assert func(mr *av1.Rule)
			rule_index  int
			path_index  int
		}
		cases := []Case{
			{
				title: "basic set should ok",
				annotations: map[string]string{
					"nginx.ingress.kubernetes.io/auth-url":    "https://$host/oauth2/auth",
					"nginx.ingress.kubernetes.io/auth-signin": "https://$host/oauth2/start?rd=$escaped_request_uri",
				},
				rule_assert: func(mr *av1.Rule) {
					assert.Equal(t, mr.Spec.Config.Auth.Forward.Url, "https://$host/oauth2/auth")
					assert.Equal(t, mr.Spec.Config.Auth.Forward.Signin, "https://$host/oauth2/start?rd=$escaped_request_uri")
				},
			},
			{
				title: "ignore path should ok",
				annotations: map[string]string{
					"nginx.ingress.kubernetes.io/auth-enable": "false",
					"nginx.ingress.kubernetes.io/auth-url":    "https://$host/oauth2/auth",
					"nginx.ingress.kubernetes.io/auth-signin": "https://$host/oauth2/start?rd=$escaped_request_uri",
				},
				rule_assert: func(mr *av1.Rule) {
					assert.Nil(t, mr.Spec.Config.Auth)
				},
			},
			{
				title: "specific path index should ok",
				annotations: map[string]string{
					"nginx.ingress.kubernetes.io/auth-url":       "https://$host/oauth2/auth",
					"nginx.ingress.kubernetes.io/auth-signin":    "https://$host/oauth2/start?rd=$escaped_request_uri",
					"alb.ingress.cpaas.io/index/0-0/auth-url":    "a.com",
					"alb.ingress.cpaas.io/index/0-0/auth-signin": "b.com",
				},
				rule_assert: func(mr *av1.Rule) {
					assert.Equal(t, mr.Spec.Config.Auth.Forward.Url, "a.com")
					assert.Equal(t, mr.Spec.Config.Auth.Forward.Signin, "b.com")
				},
			},
			{
				title: "disable should ok",
				annotations: map[string]string{
					"alb.ingress.cpaas.io/auth-enable": "false",
				},
				rule_assert: func(mr *av1.Rule) {
					assert.Nil(t, mr.Spec.Config.Auth)
				},
			},
			{
				title:       "default should ok",
				annotations: map[string]string{},
				rule_assert: func(mr *av1.Rule) {
					assert.Nil(t, mr.Spec.Config.Auth)
				},
			},
			{
				title: "basic auth should ok",
				annotations: map[string]string{
					"nginx.ingress.kubernetes.io/auth-realm":       "default",
					"nginx.ingress.kubernetes.io/auth-secret":      "cpaas-system/auth-secret",
					"nginx.ingress.kubernetes.io/auth-secret-type": "auth-file",
					"nginx.ingress.kubernetes.io/auth-type":        "basic",
				},
				rule_assert: func(mr *av1.Rule) {
					GinkgoAssertJsonEq(mr.Spec.Config.Auth.Basic, `
                    {
                        "auth_type": "basic",
                        "realm": "default",
                        "secret": "cpaas-system/auth-secret",
                        "secret_type": "auth-file",
                    }
					`, "")
				},
			},
		}
		for _, c := range cases {
			annotation := c.annotations
			a_ctl := NewAuthCtl(l, "cpaas.io")
			ing := &nv1.Ingress{
				ObjectMeta: meta_v1.ObjectMeta{
					Annotations: annotation,
				},
				Spec: nv1.IngressSpec{},
			}
			mr := &av1.Rule{
				Spec: av1.RuleSpec{
					Config: &av1.RuleConfigInCr{},
				},
			}

			a_ctl.IngressAnnotationToRule(ing, c.rule_index, c.path_index, mr)
			l.Info(c.title, "mr", PrettyCr(mr.Spec.Config.Auth))
			c.rule_assert(mr)
		}
	})

	It("to policy should ok", func() {
		type Case struct {
			title    string
			rule     *InternalRule
			refs     RefMap
			p_assert func(p *Policy)
		}
		cases := []Case{
			{
				title: "resolve configmap refs should ok",
				rule: &InternalRule{
					Config: RuleExt{
						Auth: &AuthCr{
							Forward: &ForwardAuthInCr{
								Url:              "https://$host/oauth2/auth",
								AuthHeadersCmRef: "cpaas-system/auth-cm",
							},
						},
					},
				},
				refs: RefMap{
					ConfigMap: map[client.ObjectKey]*corev1.ConfigMap{
						{
							Namespace: "cpaas-system",
							Name:      "auth-cm",
						}: {
							Data: map[string]string{
								"x-xx": "$host-$uri",
							},
						},
					},
				},
				p_assert: func(p *Policy) {
					assert.Equal(t, p.Config.Auth.Forward.Url, VarString{"https://", "$host", "/oauth2/auth"})
					assert.Equal(t, p.Config.Auth.Forward.AuthHeaders["x-xx"], VarString{"$host", "-", "$uri"})
					assert.Equal(t, p.Config.Auth.Forward.UpstreamHeaders, []string{})
				},
			},
			{
				title: "resolve signin url and redirect param should ok",
				rule: &InternalRule{
					Config: RuleExt{
						Auth: &AuthCr{
							Forward: &ForwardAuthInCr{
								Url:                 "https://$host/oauth2/auth",
								Signin:              "https://$host/oauth2/start",
								SigninRedirectParam: "xx",
							},
						},
					},
				},
				refs: RefMap{},
				p_assert: func(p *Policy) {
					assert.Equal(t, p.Config.Auth.Forward.SigninUrl, VarString{"https://", "$host", "/oauth2/start?xx=", "$pass_access_scheme", "://", "$http_host", "$escaped_request_uri"})
				},
			},
			{
				title: "basic auth, auth-file secret should work",
				rule: &InternalRule{
					Config: RuleExt{
						Auth: &AuthCr{
							Basic: &BasicAuthInCr{
								Realm:      "xx",
								Secret:     "cpaas-system/auth-file",
								SecretType: "auth-file",
								AuthType:   "basic",
							},
						},
					},
				},
				refs: RefMap{
					Secret: map[client.ObjectKey]*corev1.Secret{
						{Name: "auth-file", Namespace: "cpaas-system"}: {
							Type: corev1.SecretTypeOpaque,
							Data: map[string][]byte{
								//             foo:$apr1$qICNZ61Q$2iooiJVUAMmprq258/ChP1                   //  cspell:disable-line
								"auth": []byte("Zm9vOiRhcHIxJHFJQ05aNjFRJDJpb29pSlZVQU1tcHJxMjU4L0NoUDE"), //  cspell:disable-line
							},
						},
					},
				},
				p_assert: func(p *Policy) {
					l.Info("policy", "auth", u.PrettyJson(p.Config.Auth))
					/* cspell:disable-next-line */
					GinkgoAssertJsonEq(p.Config.Auth.Basic.Secret["foo"], `{"algorithm":"apr1","hash":"2iooiJVUAMmprq258/ChP1","name":"foo","salt":"qICNZ61Q"}`, "")
				},
			},
			{
				title: "basic auth, auth-map secret should work",
				rule: &InternalRule{
					Config: RuleExt{
						Auth: &AuthCr{
							Basic: &BasicAuthInCr{
								Realm:      "xx",
								Secret:     "cpaas-system/auth-map",
								SecretType: "auth-map",
								AuthType:   "basic",
							},
						},
					},
				},
				refs: RefMap{
					Secret: map[client.ObjectKey]*corev1.Secret{
						{Name: "auth-map", Namespace: "cpaas-system"}: {
							Type: corev1.SecretTypeOpaque,
							Data: map[string][]byte{
								"foo": []byte("JGFwcjEkcUlDTlo2MVEkMmlvb2lKVlVBTW1wcnEyNTgvQ2hQMQ"), //  cspell:disable-line
							},
						},
					},
				},
				p_assert: func(p *Policy) {
					l.Info("policy", "auth", u.PrettyJson(p.Config.Auth))
					GinkgoAssertStringEq(p.Config.Auth.Basic.Err, "", "")
					/* cspell:disable-next-line */
					GinkgoAssertJsonEq(p.Config.Auth.Basic.Secret["foo"], `{"algorithm":"apr1","hash":"2iooiJVUAMmprq258/ChP1","name":"foo","salt":"qICNZ61Q"}`, "")
				},
			},
		}
		for _, c := range cases {
			p := &Policy{
				Config: PolicyExtCfg{},
			}
			a_ctl := NewAuthCtl(l, "cpaas.io")
			a_ctl.ToPolicy(c.rule, p, c.refs)
			l.Info("policy", "p", pretty.Sprint(p))
			c.p_assert(p)
		}
	})
	It("get secret", func() {
		out, err := bcrypt.GenerateFromPassword([]byte("bar"), 14)
		GinkgoNoErr(err)
		l.Info("secret", "out", string(out))
	})

	It("parse hash should ok", func() {
		{
			cfg, err := parseHash("foo:$apr1$qICNZ61Q$2iooiJVUAMmprq258/ChP1", "") //  cspell:disable-line
			GinkgoNoErr(err)
			l.Info("xx", "x", u.PrettyJson(cfg))
			/* cspell:disable-next-line */
			GinkgoAssertJsonEq(cfg, `{"algorithm":"apr1","hash":"2iooiJVUAMmprq258/ChP1","name":"foo","salt":"qICNZ61Q"}`, "")
		}
		{
			cfg, err := parseHash("$apr1$qICNZ61Q$2iooiJVUAMmprq258/ChP1", "foo") //  cspell:disable-line
			GinkgoNoErr(err)
			l.Info("xx", "x", u.PrettyJson(cfg))
			/* cspell:disable-next-line */
			GinkgoAssertJsonEq(cfg, `{"algorithm":"apr1","hash":"2iooiJVUAMmprq258/ChP1","name":"foo","salt":"qICNZ61Q"}`, "")
		}
	})
})
