package test_utils

import otu "alauda.io/alb2/utils/test_utils"

type MockSvc struct {
	Ns   string
	Name string
	Port []int
	Ep   []string
}

func (m MockSvc) GenYaml() string {
	return otu.Template(`
apiVersion: v1
kind: Service
metadata:
  name: {{.name}} 
  namespace: {{.ns}}
spec:
  clusterIP: 10.0.0.254
  internalTrafficPolicy: Cluster
  sessionAffinity: None
  type: ClusterIP
  ipFamilies:
  - IPv4
  ipFamilyPolicy: SingleStack
  ports:
    {{ range $p := .port }}
  - name: port-{{$p}}
    port: {{$p}}
    protocol: TCP
    targetPort: {{$p}}
    {{ end }}
---
apiVersion: v1
kind: Endpoints
metadata:
  name: {{.name}} 
  namespace: {{.ns}}
subsets:
    {{ range $ip := .ep }}
- addresses:
  - ip: {{$ip}}
  ports:
    {{ range $p := $.port }}
    - name: port-{{$p}}
      port: {{$p}}
      protocol: TCP
    {{- end -}}
    {{- end -}}
`, map[string]interface{}{"name": m.Name, "ns": m.Ns, "port": m.Port, "ep": m.Ep})
}
