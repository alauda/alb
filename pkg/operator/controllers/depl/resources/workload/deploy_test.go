package workload

import (
	"reflect"
	"testing"

	a2t "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	. "alauda.io/alb2/pkg/config"
	"alauda.io/alb2/pkg/operator/config"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEqual(t *testing.T) {
	a := map[string]map[string]string{
		"2": {
			"1": "1",
			"2": "2",
		},
		"1": {
			"1": "1",
			"2": "2",
		},
	}
	b := map[string]map[string]string{
		"1": {
			"2": "2",
			"1": "1",
		},
		"2": {
			"1": "1",
			"2": "2",
		},
	}
	assert.Equal(t, reflect.DeepEqual(a, b), true, "")
}

func TestTemplate(t *testing.T) {
	// ops := SetImage("nginx", "xx")
	name := "name"
	ns := "ns"
	alb := a2t.ALB2{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}

	cfg := &config.ALB2Config{
		ALBRunConfig: ALBRunConfig{
			Name: name,
			Ns:   ns,
		},
	}
	cfg.Overwrite = config.OverwriteCfg{
		Image: []a2t.ExternalImageOverwriteConfig{
			{
				Alb:   "alb.img",
				Nginx: "xx",
			},
		},
	}
	defaultc := config.DefaultConfig()
	defaultc.LoadbalancerName = &name
	err := cfg.Merge(defaultc)
	assert.NoError(t, err)
	dep := NewTemplate(&alb, nil, cfg, config.DEFAULT_OPERATOR_CFG, ConsoleLog()).Generate()
	assert.Equal(t, "xx", dep.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, "alb.img", dep.Spec.Template.Spec.Containers[1].Image)
	t.Logf("depl %v", PrettyCr(dep))
}

func TestVolume(t *testing.T) {
	hostPathType := corev1.HostPathDirectoryOrCreate
	vol := VolumeCfgFromDepl(&appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "c1",
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/tmp",
									Name:      "test",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "test",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/tmp",
									Type: &hostPathType,
								},
							},
						},
					},
				},
			},
		},
	})
	t.Logf("vol %v", PrettyCr(vol))
	patch := `
    {
        "volumes":{
            "nginx-log":{"name":"nginx-log","hostPath":{"path":"/var/log/alb/","type":"DirectoryOrCreate"}}
        },
        "mounts":{
            "nginx":{
                "nginx-log":"/var/log/alb/"
         }
        }
    }
`
	err := patchVolumes(&vol, patch)
	assert.NoError(t, err)
	t.Logf("vol %v", PrettyCr(vol))
}
