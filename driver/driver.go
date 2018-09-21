package driver

import (
	"alauda_lb/config"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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

type NodePort struct {
	Name      string
	Labels    map[string]string
	Ports     []int
	Selector  map[string]string
	Namespace string
}

func (s Service) String() string {
	r, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "error decoding type into json: %s"}`, err)
	}

	return string(r)
}

type Driver interface {
	GetType() string
	IsHealthy() bool
	CreateNodePort(nc *NodePort) error
	DeleteNodePort(name, namespace string) error
	ListService() ([]*Service, error)
}

func GetDriver() (Driver, error) {
	switch strings.ToLower(config.Get("SCHEDULER")) {
	case config.Marathon:
		timeout, err := strconv.Atoi(config.Get("MARATHON_TIMEOUT"))
		if err != nil {
			timeout = 3
		}
		return &MarathonDriver{
			Endpoint: config.Get("MARATHON_SERVER"),
			Username: config.Get("MARATHON_USERNAME"),
			Password: config.Get("MARATHON_PASSWORD"),
			Timeout:  timeout}, nil
	case config.Kubernetes:
		timeout := config.GetInt("KUBERNETES_TIMEOUT")
		return GetKubernetesDriver(config.Get("KUBERNETES_SERVER"),
			config.Get("KUBERNETES_BEARERTOKEN"), timeout)
	}
	return nil, fmt.Errorf("Unsupport driver type: %s", config.Get("SCHEDULER"))
}
