package driver

import (
	"alauda.io/alb2/config"
	"fmt"

	"k8s.io/klog"
)

const (
	OwnerUnknown = "Unknown"
)

type Backend struct {
	InstanceID string `json:"instance_id"`
	IP         string `json:"ip"`
	Port       int    `json:"port"`
}

func (b *Backend) String() string {
	return fmt.Sprintf("%s:%d", b.IP, b.Port)
}

type ByBackend []*Backend

func (b ByBackend) Len() int {
	return len(b)
}

func (b ByBackend) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b ByBackend) Less(i, j int) bool {
	return fmt.Sprintf("%s-%d", b[i].IP, b[i].Port) < fmt.Sprintf("%s-%d", b[j].IP, b[j].Port)
}

type Service struct {
	ServiceID     string     `json:"service_id"`
	ServiceName   string     `json:"service_name"`
	NetworkMode   string     `json:"network_mode"`
	ServicePort   int        `json:"service_port"`
	ContainerPort int        `json:"container_port"`
	Backends      []*Backend `json:"backends"`
	Namespace     string     `json:"namespace"`
	Owner         string     `json:"owner"`
}

func (s *Service) String() string {
	return fmt.Sprintf("%s-%s-%d", s.Namespace, s.ServiceName, s.ContainerPort)
}

type NodePort struct {
	Name      string
	Labels    map[string]string
	Ports     []int
	Selector  map[string]string
	Namespace string
}

func GetDriver() (*KubernetesDriver, error) {
	timeout := config.GetInt("KUBERNETES_TIMEOUT")
	// TEST != "" means we are debugging or testing
	return GetKubernetesDriver(config.Get("TEST") != "", timeout)
}

// SetDebug enable debug mode. GetDriver() will return a driver with fake client
func SetDebug() {
	klog.Info("Set Debug")
	config.Set("TEST", "true")
}
