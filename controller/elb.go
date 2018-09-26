package controller

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/golang/glog"

	"alb2/config"
	"alb2/driver"
)

func strIn(str *string, container []string) bool {
	for _, s := range container {
		if s == *str {
			return true
		}
	}
	return false
}

type ElbController struct {
	RegionName      string
	AccessKey       string
	SecretAccessKey string
	ElbClient       *elb.ELB
	Ec2Client       *ec2.EC2
	Driver          *driver.KubernetesDriver
	LoadBalancers   []*LoadBalancer
}

func (ec *ElbController) init() {
	ec.RegionName = config.Get("IAAS_REGION")
	ec.AccessKey = config.Get("ACCESS_KEY")
	ec.SecretAccessKey = config.Get("SECRET_ACCESS_KEY")
	s := credentials.StaticProvider{
		Value: credentials.Value{
			AccessKeyID:     ec.AccessKey,
			SecretAccessKey: ec.SecretAccessKey,
		},
	}
	ec.ElbClient = elb.New(session.New(), &aws.Config{Region: &ec.RegionName, Credentials: credentials.NewCredentials(&s)})
	ec.Ec2Client = ec2.New(session.New(), &aws.Config{Region: &ec.RegionName, Credentials: credentials.NewCredentials(&s)})
}

func (ec *ElbController) GetLoadBalancerType() string {
	return "elb"
}

func validateService(service *driver.Service) bool {
	port := 0
	for _, backend := range service.Backends {
		if port == 0 {
			port = backend.Port
		} else if port != backend.Port {
			return false
		}
	}
	return true
}

func (ec *ElbController) GenerateConf() error {
	services, err := ec.Driver.ListService()
	if err != nil {
		return err
	}

	filteredServices := []*driver.Service{}
	for _, service := range services {
		if validateService(service) {
			filteredServices = append(filteredServices, service)
		}
	}

	loadbalancers, err := FetchLoadBalancersInfo()
	if err != nil {
		return err
	}
	filteredLoadbalancers := filterLoadbalancers(loadbalancers, "elb", "")

	merge(filteredLoadbalancers, filteredServices)
	mergedResult, _ := json.Marshal(loadbalancers)
	glog.Infof("Merged lb info %s", mergedResult)
	ec.LoadBalancers = filteredLoadbalancers
	return nil
}

func (ec *ElbController) getIdFromIP(ip []string) ([]string, error) {
	res := make([]string, 0, len(ip))
	filterName := "private-ip-address"
	ips := make([]*string, len(ip))
	for index := range ip {
		ips[index] = &ip[index]
	}
	instances, err := ec.Ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{Filters: []*ec2.Filter{{Name: &filterName, Values: ips}}})
	if err != nil {
		glog.Error(err.Error())
		return res, err
	}
	for _, r := range instances.Reservations {
		for _, i := range r.Instances {
			res = append(res, *i.InstanceId)
		}
	}
	return res, nil
}

func getBackends(ft *Frontend) []*Backend {
	if ft.BackendGroup != nil {
		return ft.BackendGroup.Backends
	} else if len(ft.Rules) > 0 {
		return ft.Rules[0].BackendGroup.Backends
	}
	return nil
}

func (ec *ElbController) ReloadLoadBalancer() error {
	configStr := ""
	for _, lb := range ec.LoadBalancers {
		configStr = configStr + lb.String()
	}
	if configStr != LastConfig {
		glog.Infof("new config is %s", configStr)
		glog.Infof("old config is %s", LastConfig)
	}

	if configStr == LastConfig && !LastFailure {
		glog.Info("Config not changed")
		if time.Now().Sub(lastCheckTime) < time.Minute {
			return nil
		}
		glog.Info("Re-sync config after a long while.")
	}
	LastConfig = configStr
	lastCheckTime = time.Now()

	var wait sync.WaitGroup
	var failure = false
	ch := make(chan int, len(ec.LoadBalancers))
	for index := range ec.LoadBalancers {
		wait.Add(1)
		ch <- index
		go func() {
			defer wait.Done()
			lb := ec.LoadBalancers[<-ch]
			output, err := ec.ElbClient.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{LoadBalancerNames: []*string{&lb.LoadBalancerID}})
			if err != nil {
				glog.Error(err.Error())
				failure = true
				return
			}
			if len(output.LoadBalancerDescriptions) == 0 {
				glog.Error(fmt.Sprintf("lb %s not exists", lb.LoadBalancerID))
				failure = true
				return
			}
			currentLoadBalancer := output.LoadBalancerDescriptions[0]
			currentListeners := currentLoadBalancer.ListenerDescriptions

			for _, ft := range lb.Frontends {
				backends := getBackends(ft)
				if backends == nil || len(backends) == 0 {
					return
				}

				exist := false
				for _, cl := range currentListeners {
					if *cl.Listener.LoadBalancerPort == int64(ft.Port) {
						var del bool
						if strings.ToLower(*cl.Listener.Protocol) == ProtocolHTTPS &&
							*cl.Listener.SSLCertificateId != ft.CertificateID {
							del = true
						}
						if del || int64(backends[0].Port) != *cl.Listener.InstancePort {
							loadBalancerPort := int64(ft.Port)
							glog.Infof("Delete listener %s", ft.String())
							_, err := ec.ElbClient.DeleteLoadBalancerListeners(&elb.DeleteLoadBalancerListenersInput{
								LoadBalancerName:  &lb.LoadBalancerID,
								LoadBalancerPorts: []*int64{&loadBalancerPort}})
							if err != nil {
								glog.Error(err.Error())
								failure = true
							}
						} else {
							exist = true
						}
						break
					}
				}

				if !exist {
					instancePort := int64(backends[0].Port)
					loadBalancerPort := int64(ft.Port)
					protocol := ft.Protocol
					instanceProtocol := ft.Protocol
					var sslCertificateID *string
					if protocol == ProtocolHTTPS {
						instanceProtocol = ProtocolHTTP
						sslCertificateID = &ft.CertificateID
					}
					glog.Infof("Add new listener %s", ft.String())
					_, err := ec.ElbClient.CreateLoadBalancerListeners(&elb.CreateLoadBalancerListenersInput{
						LoadBalancerName: &lb.LoadBalancerID,
						Listeners: []*elb.Listener{
							{
								InstancePort:     &instancePort,
								LoadBalancerPort: &loadBalancerPort,
								Protocol:         &protocol,
								InstanceProtocol: &instanceProtocol,
								SSLCertificateId: sslCertificateID,
							}}})
					if err != nil {
						glog.Error(err.Error())
						failure = true
						return
					}
				}
			}

			// Delete listeners
			for _, cl := range currentListeners {
				shouldRemove := true
				for _, ft := range lb.Frontends {
					if int64(ft.Port) == *cl.Listener.LoadBalancerPort {
						shouldRemove = false
						break
					}
				}
				if shouldRemove {
					glog.Infof("Delete listener port %v from %s", cl.Listener.LoadBalancerPort, lb.LoadBalancerID)
					_, err := ec.ElbClient.DeleteLoadBalancerListeners(&elb.DeleteLoadBalancerListenersInput{
						LoadBalancerName:  &lb.LoadBalancerID,
						LoadBalancerPorts: []*int64{cl.Listener.LoadBalancerPort}})
					if err != nil {
						glog.Error(err.Error())
						failure = true
						return
					}
				}
			}

			// Add new instances
			currentInstances := make([]string, 0, len(output.LoadBalancerDescriptions[0].Instances))
			for _, i := range output.LoadBalancerDescriptions[0].Instances {
				currentInstances = append(currentInstances, *i.InstanceId)
			}
			glog.Infof("Current instance %v in lb %s", currentInstances, lb.LoadBalancerID)
			exceptIPs := []string{}

			if len(lb.Frontends) > 0 {
				backends := getBackends(lb.Frontends[0])
				for _, i := range backends {
					exceptIPs = append(exceptIPs, i.Address)
				}
			}

			exceptInstances, err := ec.getIdFromIP(exceptIPs)
			if err != nil {
				glog.Errorf("Get id from ip failed %s", err.Error())
				failure = true
				return
			}
			glog.Infof("Expect instance %v in lb %s", exceptInstances, lb.LoadBalancerID)

			addInstances := []*elb.Instance{}
			for index := range exceptInstances {
				if !strIn(&exceptInstances[index], currentInstances) {
					addInstances = append(addInstances, &elb.Instance{InstanceId: &exceptInstances[index]})
				}
			}
			if len(addInstances) > 0 {
				_, err := ec.ElbClient.RegisterInstancesWithLoadBalancer(&elb.RegisterInstancesWithLoadBalancerInput{
					LoadBalancerName: &lb.LoadBalancerID,
					Instances:        addInstances})
				if err != nil {
					glog.Error(err.Error())
					failure = true
					return
				}
			}

			// Delete instances
			removeInstances := []*elb.Instance{}
			for index := range currentInstances {
				if !strIn(&currentInstances[index], exceptInstances) {
					removeInstances = append(removeInstances, &elb.Instance{InstanceId: &currentInstances[index]})
				}
			}
			if len(removeInstances) > 0 {
				_, err := ec.ElbClient.DeregisterInstancesFromLoadBalancer(&elb.DeregisterInstancesFromLoadBalancerInput{
					LoadBalancerName: &lb.LoadBalancerID,
					Instances:        removeInstances})
				if err != nil {
					glog.Error(err.Error())
					failure = true
					return
				}
			}

			// Update Health-Check
			// TODO: only support tcp to first port now
			// TODO: should retrieve from mirana2 later
			currentTarget := currentLoadBalancer.HealthCheck.Target
			_, healthCheckPortStr := strings.Split(*currentTarget, ":")[0], strings.Split(*currentTarget, ":")[1]
			healthCheckPort, _ := strconv.Atoi(healthCheckPortStr)
			if len(lb.Frontends) > 0 {
				backends := getBackends(lb.Frontends[0])
				if len(backends) > 0 && healthCheckPort != backends[0].Port {
					threshold := int64(2)
					interval := int64(5)
					timeout := int64(2)
					target := fmt.Sprintf("%s:%v", "TCP", backends[0].Port)
					healthCheck := elb.HealthCheck{
						Target:             &target,
						HealthyThreshold:   &threshold,
						Interval:           &interval,
						Timeout:            &timeout,
						UnhealthyThreshold: &threshold}
					glog.Infof("Update %s health-check to %s", lb.LoadBalancerID, target)
					_, err := ec.ElbClient.ConfigureHealthCheck(&elb.ConfigureHealthCheckInput{HealthCheck: &healthCheck, LoadBalancerName: &lb.LoadBalancerID})
					if err != nil {
						glog.Error(err.Error())
						failure = true
						return
					}
				}
			}
		}()
	}
	wait.Wait()
	LastFailure = failure
	return nil
}
