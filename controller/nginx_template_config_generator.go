package controller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"

	"alauda.io/alb2/config"
	"alauda.io/alb2/utils"
	"k8s.io/klog"
)

type LegacyConfig = Config
type FtConfig struct {
	Port            int
	Protocol        string
	IpV4BindAddress []string
	IpV6BindAddress []string
}

type MetisConfig struct {
	Port            int
	IpV4BindAddress []string
	IpV6BindAddress []string
}

// a config used for nginx.tml to generate nginx.conf
type NginxTemplateConfig struct {
	Name      string
	Frontends map[int]FtConfig
	Metis     MetisConfig
	TweakHash string
	Phase     string
	NginxParam
}

type NginxTemplateConfigGenerator struct {
	config LegacyConfig
}

func NewNginxTemplateConfigGenerator(cfg LegacyConfig) NginxTemplateConfigGenerator {
	return NginxTemplateConfigGenerator{config: cfg}
}

func (g NginxTemplateConfigGenerator) Generate() (NginxTemplateConfig, error) {
	ipv4, ipv6, err := GetBindIp()
	if err != nil {
		return NginxTemplateConfig{}, err
	}

	fts := make(map[int]FtConfig)
	for port, ft := range g.config.Frontends {
		fts[port] = FtConfig{
			IpV4BindAddress: ipv4,
			IpV6BindAddress: ipv6,
			Port:            ft.Port,
			Protocol:        ft.Protocol,
		}
	}

	return NginxTemplateConfig{
		Name:      g.config.Name,
		Frontends: fts,
		TweakHash: g.config.TweakHash,
		Phase:     g.config.Phase,
		Metis: MetisConfig{
			Port:            g.config.MetricsPort,
			IpV4BindAddress: ipv4,
			IpV6BindAddress: ipv6,
		},
		NginxParam: g.config.NginxParam,
	}, nil
}

func (g NginxTemplateConfigGenerator) generateFts(fts map[int]*Frontend) (map[int]FtConfig, error) {
	ipv4, ipv6, err := GetBindIp()
	if err != nil {
		return nil, err
	}
	result := make(map[int]FtConfig)
	for port, ft := range fts {
		result[port] = FtConfig{
			IpV4BindAddress: ipv4,
			IpV6BindAddress: ipv6,
			Port:            ft.Port,
			Protocol:        ft.Protocol,
		}
	}
	return result, nil
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

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func GetBindNICConfig() (BindNICConfig, error) {
	bindNICConfigFile := filepath.Join(config.Get("TWEAK_DIRECTORY"), "bind_nic.json")
	if !fileExists(bindNICConfigFile) {
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
