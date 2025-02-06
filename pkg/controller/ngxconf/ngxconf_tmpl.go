package ngxconf

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"alauda.io/alb2/utils/dirhash"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"

	"alauda.io/alb2/config"
	"alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	cus "alauda.io/alb2/pkg/controller/extctl"
	. "alauda.io/alb2/pkg/controller/ngxconf/types"
	pm "alauda.io/alb2/pkg/utils/metrics"
	"alauda.io/alb2/utils"
)

type NgxCli struct {
	drv *driver.KubernetesDriver
	log logr.Logger
	opt NgxCliOpt
	cus cus.ExtCtl
}

type NgxCliOpt struct{}

func NewNgxCli(drv *driver.KubernetesDriver, log logr.Logger, opt NgxCliOpt) NgxCli {
	return NgxCli{
		drv: drv,
		log: log,
		opt: opt,
		cus: cus.NewExtensionCtl(cus.ExtCtlCfgOpt{Log: log, Domain: drv.Opt.Domain}),
	}
}

func (c *NgxCli) GenerateNginxTemplateConfig(alb *types.LoadBalancer, phase string, cfg *config.Config) (*NginxTemplateConfig, error) {
	s := time.Now()
	defer func() {
		pm.Write("gen-nginx-conf", float64(time.Since(s).Milliseconds()))
	}()
	nginxParam := newNginxParam(cfg)
	s_bind_ip := time.Now()
	ipv4, ipv6, err := GetBindIp(cfg)
	if err != nil {
		return nil, err
	}
	pm.Write("gen-nginx-conf/bind-ip", float64(time.Since(s_bind_ip).Milliseconds()))
	fts := make(map[string]FtConfig)
	for _, ft := range alb.Frontends {
		if ft.Conflict {
			continue
		}
		fts[ft.String()] = FtConfig{
			IpV4BindAddress: ipv4,
			IpV6BindAddress: ipv6,
			Port:            int(ft.Port),
			Protocol:        ft.Protocol,
			EnableHTTP2:     nginxParam.EnableHTTP2,
			CertificateName: ft.CertificateName,
		}
	}
	// calculate hash by tweak dir
	tweakBase := cfg.GetNginxCfg().TweakDir
	hash := "default"
	s_hash := time.Now()
	if tweakBase != "" {
		hash, err = dirhash.HashDir(tweakBase, ".conf", dirhash.DefaultHash)
		if err != nil {
			c.log.Error(err, "failed to calculate hash")
			return nil, err
		}
	}
	pm.Write("gen-nginx-conf/hash-tweak", float64(time.Since(s_hash).Milliseconds()))

	resolver, err := getDnsResolver()
	if err != nil {
		return nil, err
	}

	tmpl_cfg := &NginxTemplateConfig{
		Name:      alb.Name,
		TweakBase: tweakBase,
		NginxBase: "/alb/nginx",
		RestyBase: "/usr/local/openresty",
		ShareBase: "/etc/alb2/nginx",
		Frontends: fts,
		TweakHash: hash,
		Resolver:  resolver,
		Phase:     phase,
		Metrics: MetricsConfig{
			Port:            nginxParam.MetricsPort,
			IpV4BindAddress: ipv4,
			IpV6BindAddress: ipv6,
		},
		NginxParam: nginxParam,
		Flags:      DefaulNgxTmplFlags(),
	}
	err = c.cus.UpdateNgxTmpl(tmpl_cfg, alb, cfg)
	if err != nil {
		return nil, err
	}
	return tmpl_cfg, nil
}

func getDnsResolver() (string, error) {
	f, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return "", err
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return getDnsResolverRaw(string(raw))
}

func getDnsResolverRaw(raw string) (string, error) {
	var nameservers []string
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if strings.HasPrefix(line, "nameserver") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				nameservers = append(nameservers, parts[1])
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	// dual-stack and ipv4 first
	for _, ip := range nameservers {
		if utils.IsIPv4(ip) {
			return ip, nil
		}
	}
	for _, ip := range nameservers {
		if utils.IsIPv6(ip) {
			return "[" + ip + "]", nil
		}
	}
	return "", fmt.Errorf("no nameserver found in %v", raw)
}

func NgxTmplCfgFromYaml(ngx string) (*NginxTemplateConfig, error) {
	var cfg NginxTemplateConfig
	err := yaml.Unmarshal([]byte(ngx), &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %v", err)
	}
	return &cfg, nil
}

func GetBindIp(cfg *config.Config) (ipv4Address []string, ipv6Address []string, err error) {
	bindNICConfig, err := GetBindNICConfig(cfg.TweakDir)
	if err != nil {
		return nil, nil, err
	}

	enableIpv6 := cfg.GetNginxCfg().EnableIpv6
	if len(bindNICConfig.Nic) == 0 {
		ipv4 := []string{"0.0.0.0"}
		ipv6 := []string{"[::]"}
		if !enableIpv6 {
			ipv6 = []string{}
		}
		return ipv4, ipv6, nil
	}
	networkInfo, err := GetCurrentNetwork()
	if err != nil {
		return nil, nil, err
	}
	return getBindIp(bindNICConfig, networkInfo, enableIpv6)
}

func getBindIp(bindNICConfig BindNICConfig, networkInfo NetWorkInfo, enableIpv6 bool) (ipv4Address []string, ipv6Address []string, err error) {
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

	if nicMap["lo"] {
		ipv4Address = append(ipv4Address, "127.0.0.1")
		if enableIpv6 {
			ipv6Address = append(ipv6Address, "::1")
		}
	}

	if len(ipv4Address) == 0 {
		ipv4Address = append(ipv4Address, "0.0.0.0")
	}
	if enableIpv6 && len(ipv6Address) == 0 {
		ipv6Address = append(ipv6Address, "[::]")
	}

	ipv4Address = utils.StrListRemoveDuplicates(ipv4Address)
	ipv6Address = utils.StrListRemoveDuplicates(ipv6Address)
	sort.Strings(ipv4Address)
	sort.Strings(ipv6Address)
	return ipv4Address, ipv6Address, nil
}

type InterfaceInfo struct {
	Name        string
	IpV4Address []string
	IpV6Address []string
}

type NetWorkInfo = map[string]InterfaceInfo

// TODO  GetCurrentNetwork maybe slow (80ms),但是标准库中获取interface本质上也是先获取所有的nic
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

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return BindNICConfig{}, err
	}
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
