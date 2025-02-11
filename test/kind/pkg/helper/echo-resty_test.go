package helper

import (
	"os"
	"testing"

	"alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
)

func TestEchoResty(t *testing.T) {
	t.SkipNow()
	print(EchoRestyTemplate)
	base := InitBase()
	cfg, err := RESTFromKubeConfigFile(os.Getenv("KUBECONFIG"))
	assert.NoError(t, err)
	image := os.Getenv("ALB_IMAGE")
	raw := `
		access_log  /dev/stdout  ;
		error_log   /dev/stdout  info;
		location / {
			content_by_lua_block {
				ngx.say("1")
			}
		}
	`
	e, err := NewEchoResty(base, cfg, log.L()).Deploy(EchoCfg{Name: "echo-resty", Image: image, Raw: raw, PodPort: "80"})
	assert.NoError(t, err)
	print(e.GetIp())
	assert.NoError(t, err)
	k := NewKubectl(base, cfg, log.L())
	pods, err := e.GetRunningPods()
	assert.NoError(t, err)
	pod := pods[0]
	out := k.AssertKubectl("exec", pod.Name, "--", "curl", "-s", "http://127.0.0.1:80")
	assert.Equal(t, "1", out)

	raw = `
		access_log  /dev/stdout  ;
		error_log   /dev/stdout  info;
		location / {
			content_by_lua_block {
				ngx.say("2")
			}
		}
	`
	e, err = NewEchoResty(base, cfg, log.L()).Deploy(EchoCfg{Name: "echo-resty", Image: image, Raw: raw, PodPort: "80"})
	assert.NoError(t, err)
	out = k.AssertKubectl("exec", pod.Name, "--", "curl", "-s", "http://127.0.0.1:80")
	assert.Equal(t, "2", out)
}
