package controller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sort"
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
	CertificateName string
	IpV4BindAddress []string
	IpV6BindAddress []string
}

type MetricsConfig struct {
	Port            int
	IpV4BindAddress []string
	IpV6BindAddress []string
}

// a config used for nginx.tmpl to generate nginx.conf
type NginxTemplateConfig struct {
	Name      string
	Frontends map[string]FtConfig
	Metrics   MetricsConfig
	TweakHash string
	Phase     string
	NginxParam
}

func GenerateNginxTemplateConfig(alb *LoadBalancer, phase string, nginxParam NginxParam, cfg *config.Config) (*NginxTemplateConfig, error) {
	ipv4, ipv6, err := GetBindIp(cfg)
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
			Port:            int(ft.Port),
			Protocol:        ft.Protocol,
			CertificateName: ft.CertificateName,
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

func GetBindIp(cfg *config.Config) (ipv4Address []string, ipv6Address []string, err error) {
	bindNICConfig, err := GetBindNICConfig(cfg.TweakDir)
	if err != nil {
		return nil, nil, err
	}

	networkInfo, err := GetCurrentNetwork()
	if err != nil {
		return nil, nil, err
	}
	return getBindIp(bindNICConfig, networkInfo, cfg.GetNginxCfg().EnableIpv6)
}

func getBindIp(bindNICConfig BindNICConfig, networkInfo NetWorkInfo, enableIpv6 bool) (ipv4Address []string, ipv6Address []string, err error) {
	if len(bindNICConfig.Nic) == 0 {
		klog.Info("[bind_nic] without config bind 0.0.0.0")
		ipv4 := []string{"0.0.0.0"}
		ipv6 := []string{"[::]"}
		if !enableIpv6 {
			ipv6 = []string{}
		}
		return ipv4, ipv6, nil
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
		if !enableIpv6 {
			continue
		}
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
	if enableIpv6 && len(ipv6Address) == 0 {
		klog.Info("[bind_nic] could not find any ipv6 address and enableIpv6 bind [::]")
		ipv6Address = append(ipv6Address, "[::]")
	}
	ipv4Address = utils.StrListRemoveDuplicates(ipv4Address)
	ipv6Address = utils.StrListRemoveDuplicates(ipv6Address)
	sort.Strings(ipv4Address)
	sort.Strings(ipv6Address)
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

func GetBindNICConfig(base string) (BindNICConfig, error) {
	bindNICConfigFile := filepath.Join(base, "bind_nic.json")
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
