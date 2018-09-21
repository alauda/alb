package driver

import (
	"alauda_lb/config"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gambol99/go-marathon"
	"github.com/golang/glog"
)

type MarathonDriver struct {
	Endpoint string
	Username string
	Password string
	Timeout  int
}

func (mc *MarathonDriver) generateMarathonClient() (marathon.Marathon, error) {
	cf := marathon.NewDefaultConfig()
	cf.URL = mc.Endpoint
	cf.HTTPClient = &http.Client{
		Timeout: time.Duration(mc.Timeout) * time.Second,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   time.Duration(mc.Timeout) * time.Second,
				KeepAlive: time.Duration(mc.Timeout) * time.Second,
			}).Dial,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	if mc.Username != "" && mc.Password != "" {
		cf.HTTPBasicAuthUser = mc.Username
		cf.HTTPBasicPassword = mc.Password
	}
	return marathon.NewClient(cf)
}

func (mc *MarathonDriver) fetchMarathonServices() (*marathon.Applications, error) {
	client, err := mc.generateMarathonClient()
	if err != nil {
		glog.Errorf("Failed to create a client for marathon, error: %s", err)
		return nil, err
	}
	params := url.Values{}
	params.Set("embed", "app.tasks")
	return client.Applications(params)
}

func (mc *MarathonDriver) parseService(app marathon.Application) ([]*Service, error) {

	serviceEndpoints := []*Service{}
	if app.Container == nil || app.Container.Docker == nil {
		return serviceEndpoints, nil
	}

	if len(app.ID) < 37 {
		glog.Warningf("%s is not a valid service", app.ID)
		return serviceEndpoints, nil
	}

	if app.Container.Docker.Network == "BRIDGE" {
		if app.Container.Docker.PortMappings == nil {
			return serviceEndpoints, nil
		} else {
			for index, port := range *(app.Container.Docker.PortMappings) {
				appID := app.ID[1:37]
				backends := make([]*Backend, 0, len(app.Tasks))
				for _, t := range app.Tasks {
					addrs, err := net.LookupIP(t.Host)
					if err != nil || len(addrs) == 0 {
						glog.Errorf("%s can not be resolve to IP", t.Host)
						return serviceEndpoints, fmt.Errorf("%s can not be resolve to IP", t.Host)
					}
					ip := ""
					for _, addr := range addrs {
						if addr.To4() != nil {
							ip = addr.String()
							break
						}
					}
					if ip == "" {
						return serviceEndpoints, fmt.Errorf("%s can not be resolve to v4 IP", t.Host)
					}
					backends = append(backends, &Backend{IP: ip, Port: t.Ports[index], InstanceID: t.ID})
				}
				serviceEndpoint := &Service{
					ServiceID:     appID,
					ServiceName:   "",
					ContainerPort: port.ContainerPort,
					Backends:      backends,
					NetworkMode:   "BRIDGE"}
				serviceEndpoints = append(serviceEndpoints, serviceEndpoint)
			}
		}

	} else if app.Container.Docker.Network == "HOST" {
		appID := app.ID[1:37]
		backends := make([]*Backend, 0, len(app.Tasks))
		for _, t := range app.Tasks {
			addrs, err := net.LookupIP(t.Host)
			if err != nil || len(addrs) == 0 {
				glog.Errorf("%s can not be resolve to IP", t.Host)
				return serviceEndpoints, fmt.Errorf("%s can not be resolve to IP", t.Host)
			}
			ip := ""
			for _, addr := range addrs {
				if addr.To4() != nil {
					ip = addr.String()
					break
				}
			}
			if ip == "" {
				return serviceEndpoints, fmt.Errorf("%s can not be resolve to v4 IP", t.Host)
			}
			backends = append(backends, &Backend{IP: ip, Port: 0, InstanceID: t.ID})
		}
		serviceEndpoint := &Service{
			ServiceID:     appID,
			ServiceName:   "",
			ContainerPort: 0,
			Backends:      backends,
			NetworkMode:   "HOST"}
		serviceEndpoints = append(serviceEndpoints, serviceEndpoint)
	}

	return serviceEndpoints, nil
}

func (mc *MarathonDriver) ListService() ([]*Service, error) {
	marathonApplications, err := mc.fetchMarathonServices()
	if err != nil {
		return nil, err
	}

	serviceEndpoints := []*Service{}
	for _, app := range marathonApplications.Apps {
		se, err := mc.parseService(app)
		if err == nil {
			serviceEndpoints = append(serviceEndpoints, se...)
		}
	}
	glog.Info("Services from Marathon")
	return serviceEndpoints, nil
}

func (mc *MarathonDriver) GetType() string {
	return config.Marathon
}

func (mc *MarathonDriver) IsHealthy() bool {
	client, err := mc.generateMarathonClient()
	if err != nil {
		glog.Errorf("Failed to create a client for marathon, error: %s", err)
		return false
	}
	ping, err := client.Ping()
	if !ping || err != nil {
		return false
	} else {
		return true
	}
}

func (mc *MarathonDriver) CreateNodePort(np *NodePort) error {
	return nil
}

func (mc *MarathonDriver) DeleteNodePort(name, namespace string) error {
	return nil
}
