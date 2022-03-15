package controller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"

	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/utils"
	"k8s.io/klog/v2"
)

type LegacyConfig = Config
type FtConfig struct {
	Port            int
	Protocol        albv1.FtProtocol
	IpV4BindAddress []string
	IpV6BindAddress []string
}

type MetricsConfig struct {
	Port            int
	IpV4BindAddress []string
	IpV6BindAddress []string
}

// a config used for nginx.tml to generate nginx.conf
type NginxTemplateConfig struct {
	Name      string
	Frontends map[string]FtConfig
	Metrics   MetricsConfig
	TweakHash string
	Phase     string
	NginxParam
}

type NginxTemplateConfigGenerator struct {
	config LegacyConfig
}

func GenerateNginxTemplateConfig(alb *LoadBalancer, phase string, nginxParam NginxParam) (*NginxTemplateConfig, error) {
	ipv4, ipv6, err := GetBindIp()
	if err != nil {
		return nil, err
	}
	fts := make(map[string]FtConfig)
	for _, ft := range alb.Frontends {
		if ft.Conflict {
			continue
		}
		fts[fmt.Sprintf("%d-%s", ft.Port, ft.Protocol)] = FtConfig{
			IpV4BindAddress: ipv4,
			IpV6BindAddress: ipv6,
			Port:            ft.Port,
			Protocol:        ft.Protocol,
		}
	}

	return &NginxTemplateConfig{
		Name:      alb.Name,
		Frontends: fts,
		TweakHash: alb.TweakHash,
		Phase:     phase,
		Metrics: MetricsConfig{
			Port:            nginxParam.MetricsPort,
			IpV4BindAddress: ipv4,
			IpV6BindAddress: ipv6,
		},
		NginxParam: nginxParam,
	}, nil
}

func GetBindIp() (ipv4Address []string, ipv6Address []string, err error) {
	bindNICConfig, err := GetBindNICConfig()
	if err != nil {
		return nil, nil, err
	}

	networkInfo, err := GetCurrentNetwork()
	if err != nil {
		return nil, nil, err
	}
	return getBindIp(bindNICConfig, networkInfo)
}

func getBindIp(bindNICConfig BindNICConfig, networkInfo NetWorkInfo) (ipv4Address []string, ipv6Address []string, err error) {
	if len(bindNICConfig.Nic) == 0 {
		klog.Info("[bind_nic] without config bind 0.0.0.0")
		return []string{"0.0.0.0"}, []string{"[::]"}, nil
	}

	ipv4Address = []string{}
	ipv6Address = []string{}

	nicMap := map[string]bool{}
	for _, nic := range bindNICConfig.Nic {
		nicMap[nic] = true
	}
	for name, iface := range networkInfo {
		if !nicMap[name] {
			continue
		}
		ipv4Address = append(ipv4Address, iface.IpV4Address...)
		for _, ipv6Addr := range iface.IpV6Address {
			if !utils.IsIPv6Link(ipv6Addr) {
				ipv6Address = append(ipv6Address, fmt.Sprintf("[%s]", ipv6Addr))
			}
		}
	}
	if len(ipv4Address) == 0 {
		klog.Info("[bind_nic] could not find any ipv4 address bind 0.0.0.0")
		ipv4Address = append(ipv4Address, "0.0.0.0")
	}
	if config.GetBool("EnableIPV6") && len(ipv6Address) == 0 {
		klog.Info("[bind_nic] could not find any ipv6 address and enableIpv6 bind [::]")
		ipv6Address = append(ipv6Address, "[::]")
	}

	klog.Infof("[bind_nic] bind ipv4 %v ip v6 %v", ipv4Address, ipv6Address)
	return ipv4Address, ipv6Address, nil
}

type InterfaceInfo struct {
	Name        string
	IpV4Address []string
	IpV6Address []string
}

type NetWorkInfo = map[string]InterfaceInfo

func GetCurrentNetwork() (NetWorkInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	network := make(NetWorkInfo)
	for _, inter := range ifaces {
		name := inter.Name
		var ipv4Address []string
		var ipv6Address []string
		addrs, err := inter.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				ip := ipnet.IP.String()
				if utils.IsIPv4(ip) {
					ipv4Address = append(ipv4Address, ip)
				}
				if utils.IsIPv6(ip) {
					ipv6Address = append(ipv6Address, ip)
				}
			}
		}
		network[name] = InterfaceInfo{
			Name:        name,
			IpV4Address: ipv4Address,
			IpV6Address: ipv6Address,
		}
	}
	return network, nil
}

type BindNICConfig struct {
	Nic []string `json:"nic"`
}

func GetBindNICConfig() (BindNICConfig, error) {
	bindNICConfigFile := filepath.Join(config.Get("TWEAK_DIRECTORY"), "bind_nic.json")
	exist, err := utils.FileExists(bindNICConfigFile)
	if err != nil {
		return BindNICConfig{Nic: []string{}}, err
	}
	if !exist {
		return BindNICConfig{Nic: []string{}}, nil
	}

	jsonFile, err := os.Open(bindNICConfigFile)
	if err != nil {
		return BindNICConfig{Nic: []string{}}, nil
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	jsonStr := string(byteValue)
	if len(strings.TrimSpace(jsonStr)) == 0 {
		return BindNICConfig{}, nil
	}
	var cfg BindNICConfig
	err = json.Unmarshal(byteValue, &cfg)
	if cfg.Nic == nil {
		cfg.Nic = []string{}
	}
	return cfg, err
}
