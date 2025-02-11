package ingressnginx

import (
	"embed"
	"os"

	. "alauda.io/alb2/test/kind/pkg/helper"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	"github.com/pborman/indent"
	"k8s.io/client-go/rest"
)

//go:embed lua_snip/*
var EMBED_LUA_SNIP embed.FS

type AuthResty struct {
	l   logr.Logger
	cfg *rest.Config
	e   *Echo
}

func NewAuthResty(l logr.Logger, cfg *rest.Config) (*AuthResty, error) {
	l.Info("init echo resty here")
	state_lua, err := EMBED_LUA_SNIP.ReadFile("lua_snip/state.lua")
	if err != nil {
		return nil, err
	}
	auth_lua, err := EMBED_LUA_SNIP.ReadFile("lua_snip/auth.lua")
	if err != nil {
		return nil, err
	}
	app_lua, err := EMBED_LUA_SNIP.ReadFile("lua_snip/app.lua")
	if err != nil {
		return nil, err
	}
	pad := "                  "
	auth_and_upstream_raw := Template(`
            access_log  /dev/stdout  ;
            error_log   /dev/stdout  info;
            location /state {
                content_by_lua_block {
                  {{.state_lua}}
                }
            }
            location /auth {
                content_by_lua_block {
                  {{.auth_lua}}
                }
            }
            location / {
                content_by_lua_block {
                  {{.app_lua}}
                }
            }
        `, map[string]interface{}{
		"state_lua": indent.String(pad, string(state_lua)),
		"auth_lua":  indent.String(pad, string(auth_lua)),
		"app_lua":   indent.String(pad, string(app_lua)),
	})

	echo, err := NewEchoResty("", cfg, l).Deploy(EchoCfg{Name: "auth-server", Image: os.Getenv("ALB_IMAGE"), Ip: "v4", Raw: auth_and_upstream_raw, PodPort: "80", PodHostPort: "60080"})
	if err != nil {
		return nil, err
	}
	echo_ip, err := echo.GetIp()
	if err != nil {
		return nil, err
	}
	l.Info("echo", "echo ip", echo_ip)

	echo_host_ip, err := echo.GetHostIp()
	if err != nil {
		return nil, err
	}
	l.Info("echo", "echo host ip", echo_host_ip)
	return &AuthResty{
		l:   l,
		cfg: cfg,
		e:   echo,
	}, nil
}

func (a *AuthResty) Drop() error {
	return a.e.Drop()
}

func (a *AuthResty) GetIp() (string, error) {
	return a.e.GetIp()
}

func (a *AuthResty) GetHostIp() (string, error) {
	return a.e.GetHostIp()
}

func (a *AuthResty) GetIpAndHostIp() (string, string, error) {
	ip, err := a.GetIp()
	if err != nil {
		return "", "", err
	}
	host_ip, err := a.GetHostIp()
	if err != nil {
		return "", "", err
	}
	return ip, host_ip, nil
}
