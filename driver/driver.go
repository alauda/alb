package driver

import (
	"alauda_lb/config"
	"fmt"
)

const (
	OwnerUnknown = "Unknown"
)

type Backend struct {
	InstanceID string `json:"instance_id"`
	IP         string `json:"ip"`
	Port       int    `json:"port"`
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

type Driver interface {
	GetType() string
	IsHealthy() bool
	CreateNodePort(nc *NodePort) error
	DeleteNodePort(name, namespace string) error
	ListService() ([]*Service, error)
}

func GetDriver() (*KubernetesDriver, error) {
	timeout := config.GetInt("KUBERNETES_TIMEOUT")
	return GetKubernetesDriver(config.Get("KUBERNETES_SERVER"),
		config.Get("KUBERNETES_BEARERTOKEN"), timeout)
}
