package controller

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/denverdino/aliyungo/common"
	"github.com/denverdino/aliyungo/ecs"
	"github.com/denverdino/aliyungo/slb"
	"github.com/golang/glog"
	set "gopkg.in/fatih/set.v0"

	"alauda_lb/config"
	"alauda_lb/driver"
)

const InvalidBackendPort = 1

type SlbController struct {
	RegionName      string
	AccessKey       string
	SecretAccessKey string
	SlbClient       *slb.Client
	EcsClient       *ecs.Client
	Driver          driver.Driver
	LoadBalancers   []*LoadBalancer

	ip2ID       map[string]string
	name2ID     map[string]string
	vsgToDelete []string
}

type DescribeListenerAttribute struct {
	ListenerPort     int
	ListenerProtocol string
	BackendPort      int
	VServerGroupID   string
	CertificateID    string
	Status           slb.ListenerStatus
}

type AliBackendServer struct {
	ServerID string
	Port     int
	Weight   int
}

func (sc *SlbController) init() {
	sc.RegionName = config.Get("IAAS_REGION")
	sc.AccessKey = config.Get("ACCESS_KEY")
	sc.SecretAccessKey = config.Get("SECRET_ACCESS_KEY")
	sc.SlbClient = slb.NewClient(sc.AccessKey, sc.SecretAccessKey)
	sc.EcsClient = ecs.NewClient(sc.AccessKey, sc.SecretAccessKey)

	sc.ip2ID = make(map[string]string)
	sc.name2ID = make(map[string]string)
}

func (sc *SlbController) GetLoadBalancerType() string {
	return "slb"
}

func (sc *SlbController) GenerateConf() error {
	loadbalancers, err := FetchLoadBalancersInfo()
	if err != nil {
		return err
	}
	loadbalancers = filterLoadbalancers(loadbalancers, "slb", "")
	if err != nil {
		return err
	}
	services, err := sc.Driver.ListService()
	if err != nil {
		return err
	}
	merge(loadbalancers, services)
	sc.LoadBalancers = loadbalancers
	return nil
}

func (sc *SlbController) updateListenerV2(lb *LoadBalancer) error {
	config := generateConfig(lb)
	lbAttrs, err := sc.SlbClient.DescribeLoadBalancerAttribute(lb.LoadBalancerID)
	if err != nil {
		glog.Error(err)
		return err
	}
	for _, lpp := range lbAttrs.ListenerPortsAndProtocol.ListenerPortAndProtocol {
		f, ok := config.Frontends[lpp.ListenerPort]
		if ok && f.Protocol == lpp.ListenerProtocol {
			f.ready = true
		} else {
			glog.Infof("Delete listener port %d on LB %s", lpp.ListenerPort, lb.Name)
			if err := sc.SlbClient.DeleteLoadBalancerListener(lb.LoadBalancerID, lpp.ListenerPort); err != nil {
				glog.Error(err)
				return err
			}
			continue
		}
		if f.Protocol == ProtocolHTTPS {
			// for an https listener, cerfificated id should be compared.
			desc, err := sc.describeLoadBalancerListener(lb.LoadBalancerID, f.Port, f.Protocol)
			if err != nil {
				glog.Error(err)
				return err
			}
			if desc.CertificateID != f.CertificateID {
				f.ready = false
				glog.Infof("Delete listener port %d on LB %s", lpp.ListenerPort, lb.Name)
				if err := sc.SlbClient.DeleteLoadBalancerListener(lb.LoadBalancerID, lpp.ListenerPort); err != nil {
					glog.Error(err)
					return err
				}
			}
		}
	}
	for _, f := range lb.Frontends {
		if f.ready {
			continue
		}
		if len(f.Rules) == 0 && f.BackendGroup == nil {
			continue
		}
		glog.Infof("create listener port %d on LB %s", f.Port, lb.Name)
		certificateID := ""
		if f.Protocol == ProtocolHTTPS {
			certificateID = f.CertificateID
		}
		// backend server port is not used but aliyun API required, set to an invalid value "1".
		if err := sc.createLoadBalancerListener(lb.LoadBalancerID, f.Port, InvalidBackendPort, f.Protocol, certificateID); err != nil {
			glog.Error(err)
			return err
		}
		f.ready = true
	}

	return nil
}

func (sc *SlbController) isSameBackend(lb *LoadBalancer, vBackends slb.VBackendServers, bg *BackendGroup) bool {
	if len(vBackends.BackendServer) != len(bg.Backends) {
		return false
	}
	newIPs := set.New()
	for _, bs := range bg.Backends {
		if _, ok := sc.ip2ID[bs.Address]; !ok {
			newIPs.Add(bs.Address)
		}
	}
	ipList := []string{}
	for _, ip := range newIPs.List() {
		ipList = append(ipList, ip.(string))
	}
	if len(ipList) > 0 {
		ids, err := sc.getIdFromIP(ipList)
		if err != nil {
			glog.Error(err)
			return false
		}
		if len(ids) != len(ipList) {
			glog.Errorf("query %v ips, but get %v ids.", newIPs, ids)
			return false
		}
		for idx, ip := range ipList {
			sc.ip2ID[ip] = ids[idx]
		}
	}
	for _, bs := range bg.Backends {
		id, ok := sc.ip2ID[bs.Address]
		if !ok {
			glog.Errorf("Can't find ID of instance %s", bs.Address)
			continue
		}
		found := false
		for _, vs := range vBackends.BackendServer {
			if id == vs.ServerId &&
				bs.Port == vs.Port &&
				bs.Weight == vs.Weight {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (sc *SlbController) addNewVServerGroup(loadBalancerID, name string, bg *BackendGroup) (string, error) {
	glog.Infof("Add vsg %s for lb %s.", name, loadBalancerID)
	newIPs := set.New()
	for _, bs := range bg.Backends {
		if _, ok := sc.ip2ID[bs.Address]; !ok {
			newIPs.Add(bs.Address)
		}
	}
	ipList := []string{}
	for _, ip := range newIPs.List() {
		ipList = append(ipList, ip.(string))
	}
	if len(ipList) > 0 {
		ids, err := sc.getIdFromIP(ipList)
		if err != nil {
			return "", err
		}
		if len(ipList) != len(ids) {
			err = fmt.Errorf("query %d ip but get %d id", len(ipList), len(ids))
			glog.Error(err)
			return "", err
		}
		for idx, ip := range ipList {
			sc.ip2ID[ip] = ids[idx]
		}
	}

	var vServerGroupID string
	backendServers := []AliBackendServer{}
	for _, bs := range bg.Backends {
		backendServers = append(
			backendServers,
			AliBackendServer{
				ServerID: sc.ip2ID[bs.Address],
				Port:     bs.Port,
				Weight:   bs.Weight,
			})
		if len(backendServers) == 20 {
			bsJSON, err := json.Marshal(backendServers)
			if err != nil {
				panic(err)
			}
			if vServerGroupID == "" {
				glog.Infof("Create vsg %s on LB %s of region %s with %s", name, loadBalancerID, sc.RegionName, bsJSON)
				resp, err := sc.SlbClient.CreateVServerGroup(
					&slb.CreateVServerGroupArgs{
						LoadBalancerId:   loadBalancerID,
						RegionId:         common.Region(sc.RegionName),
						VServerGroupName: name,
						BackendServers:   string(bsJSON),
					},
				)
				if err != nil {
					glog.Error(err)
					return "", err
				}
				vServerGroupID = resp.VServerGroupId
			} else {
				glog.Infof("Add vsg %s on LB %s of region %s with %s", name, loadBalancerID, sc.RegionName, bsJSON)
				_, err := sc.SlbClient.AddVServerGroupBackendServers(
					&slb.AddVServerGroupBackendServersArgs{
						LoadBalancerId:   loadBalancerID,
						RegionId:         common.Region(sc.RegionName),
						VServerGroupName: name,
						VServerGroupId:   vServerGroupID,
						BackendServers:   string(bsJSON),
					},
				)
				if err != nil {
					glog.Error(err)
					return "", err
				}
			}
			backendServers = []AliBackendServer{}
		}
	}
	if len(backendServers) == 0 {
		return vServerGroupID, nil
	}
	bsJSON, err := json.Marshal(backendServers)
	if err != nil {
		panic(err)
	}
	if vServerGroupID == "" {
		glog.Infof("Create vsg %s on LB %s of region %s with %s", name, loadBalancerID, sc.RegionName, bsJSON)
		resp, err := sc.SlbClient.CreateVServerGroup(
			&slb.CreateVServerGroupArgs{
				LoadBalancerId:   loadBalancerID,
				RegionId:         common.Region(sc.RegionName),
				VServerGroupName: name,
				BackendServers:   string(bsJSON),
			},
		)
		if err != nil {
			glog.Error(err)
			return "", err
		}
		vServerGroupID = resp.VServerGroupId
	} else {
		glog.Infof("Add vsg %s on LB %s of region %s with %s", name, loadBalancerID, sc.RegionName, bsJSON)
		_, err := sc.SlbClient.AddVServerGroupBackendServers(
			&slb.AddVServerGroupBackendServersArgs{
				LoadBalancerId:   loadBalancerID,
				RegionId:         common.Region(sc.RegionName),
				VServerGroupName: name,
				VServerGroupId:   vServerGroupID,
				BackendServers:   string(bsJSON),
			},
		)
		if err != nil {
			glog.Error(err)
			return "", err
		}
	}
	return vServerGroupID, nil
}

func (sc *SlbController) syncNewVServerGroup(lb *LoadBalancer, vsName, vsID string, vBackends slb.VBackendServers, bg *BackendGroup) error {
	ipList := []string{}
	for _, bs := range bg.Backends {
		ipList = append(ipList, bs.Address)
	}
	if len(ipList) > 0 {
		ids, err := sc.getIdFromIP(ipList)
		if err != nil {
			glog.Error(err)
			return err
		}
		if len(ids) != len(ipList) {
			glog.Errorf("query %v ips, but get %v ids.", ipList, ids)
			return fmt.Errorf("query %v ips, but get %v ids", ipList, ids)
		}
		for idx, ip := range ipList {
			sc.ip2ID[ip] = ids[idx]
		}
	}

	expection := make(map[string]AliBackendServer)
	for _, bs := range bg.Backends {
		id, ok := sc.ip2ID[bs.Address]
		if !ok {
			glog.Errorf("Can't find ID of instance %s", bs.Address)
			continue
		}
		abs := AliBackendServer{
			ServerID: id,
			Port:     bs.Port,
			Weight:   bs.Weight,
		}
		expection[fmt.Sprintf("%s:%d", id, bs.Port)] = abs
	}
	oldServers := []AliBackendServer{}
	modifiedServers := []AliBackendServer{}
	for _, vs := range vBackends.BackendServer {
		key := fmt.Sprintf("%s:%d", vs.ServerId, vs.Port)
		if abs, ok := expection[key]; ok {
			delete(expection, key)
			if abs.Weight != vs.Weight {
				modifiedServers = append(modifiedServers, abs)
			}
		} else {
			oldServers = append(oldServers,
				AliBackendServer{
					ServerID: vs.ServerId,
					Port:     vs.Port,
					Weight:   vs.Weight,
				},
			)
		}
	}

	newServers := []AliBackendServer{}
	for _, abs := range expection {
		newServers = append(newServers, abs)
	}

	oldServersJson, err := json.Marshal(oldServers)
	if err != nil {
		glog.Error(err)
		return err
	}

	newServersJson, err := json.Marshal(newServers)
	if err != nil {
		glog.Error(err)
		return err
	}

	modifiedServersJson, err := json.Marshal(modifiedServers)
	if err != nil {
		glog.Error(err)
		return err
	}

	if len(newServers) > 0 {
		if len(oldServers) > 0 {
			glog.Infof("ModifyVServerGroupBackendServers %s, from %s to %s ", vsID, string(oldServersJson), string(newServersJson))
			_, err = sc.SlbClient.ModifyVServerGroupBackendServers(&slb.ModifyVServerGroupBackendServersArgs{
				VServerGroupId:    vsID,
				RegionId:          common.Region(sc.RegionName),
				OldBackendServers: string(oldServersJson),
				NewBackendServers: string(newServersJson),
			})
		} else {
			glog.Infof("AddVServerGroupBackendServers %s, %s ", vsID, string(newServersJson))
			_, err = sc.SlbClient.AddVServerGroupBackendServers(&slb.AddVServerGroupBackendServersArgs{
				RegionId:       common.Region(sc.RegionName),
				VServerGroupId: vsID,
				BackendServers: string(newServersJson),
			})
		}
	} else {
		if len(oldServers) > 0 {
			glog.Infof("RemoveVServerGroupBackendServers %s, %s ", vsID, string(oldServersJson))
			_, err = sc.SlbClient.RemoveVServerGroupBackendServers(&slb.RemoveVServerGroupBackendServersArgs{
				RegionId:       common.Region(sc.RegionName),
				VServerGroupId: vsID,
				BackendServers: string(oldServersJson),
			})
		}
	}
	if err != nil {
		glog.Error(err)
		return err
	}

	if len(modifiedServers) > 0 {
		glog.Infof("")
		_, err = sc.SlbClient.SetVServerGroupAttribute(&slb.SetVServerGroupAttributeArgs{
			RegionId:         common.Region(sc.RegionName),
			VServerGroupId:   vsID,
			VServerGroupName: vsName,
			BackendServers:   string(modifiedServersJson),
		})
		if err != nil {
			glog.Error(err)
			return err
		}
	}
	return nil
}

func (sc *SlbController) updateVServerGroup(lb *LoadBalancer) error {
	cfg := generateConfig(lb)
	services := make(map[string]*BackendGroup)
	for _, bg := range cfg.BackendGroup {
		services[bg.Name] = bg
	}
	name2ID := make(map[string]string)
	toDel := []string{}
	VSGresp, err := sc.SlbClient.DescribeVServerGroups(
		&slb.DescribeVServerGroupsArgs{
			LoadBalancerId: lb.LoadBalancerID,
			RegionId:       common.Region(sc.RegionName),
		},
	)
	if err != nil {
		glog.Error(err)
		return err
	}
	for _, vsg := range VSGresp.VServerGroups.VServerGroup {
		if bg, ok := services[vsg.VServerGroupName]; ok {
			VsgAttr, err := sc.SlbClient.DescribeVServerGroupAttribute(
				&slb.DescribeVServerGroupAttributeArgs{
					VServerGroupId: vsg.VServerGroupId,
					RegionId:       common.Region(sc.RegionName),
				},
			)
			if err != nil {
				glog.Error(err)
				return err
			}
			if !sc.isSameBackend(lb, VsgAttr.BackendServers, bg) {
				err = sc.syncNewVServerGroup(lb, vsg.VServerGroupName, vsg.VServerGroupId, VsgAttr.BackendServers, bg)
				if err != nil {
					glog.Error(err)
					return err
				}
			}
			name2ID[vsg.VServerGroupName] = vsg.VServerGroupId
			delete(services, vsg.VServerGroupName)
		} else {
			toDel = append(toDel, vsg.VServerGroupId)
		}
	}
	// vservergroup must be deleted after related rules deleted.
	sc.vsgToDelete = toDel

	//Add new groupst
	for name, bg := range services {
		vServerGroupID, err := sc.addNewVServerGroup(lb.LoadBalancerID, name, bg)
		if err != nil {
			return err
		}
		name2ID[name] = vServerGroupID
	}
	sc.name2ID = name2ID
	return nil
}

func (sc *SlbController) getRuleName(rule *Rule) string {
	vgroupName := rule.BackendGroup.Name
	vid := sc.name2ID[vgroupName]
	h := md5.New()
	io.WriteString(h, rule.Domain)
	io.WriteString(h, rule.URL)
	io.WriteString(h, vid)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (sc *SlbController) updateRules(loadBalancerID string, frontend *Frontend) error {
	if frontend.Protocol == ProtocolHTTP || frontend.Protocol == ProtocolHTTPS {
		resp, err := sc.SlbClient.DescribeRules(
			&slb.DescribeRulesArgs{
				RegionId:       common.Region(sc.RegionName),
				LoadBalancerId: loadBalancerID,
				ListenerPort:   frontend.Port,
			})
		if err != nil {
			glog.Error(err)
			return err
		}
		needUpdate := false
		offset := 0
		for idx, rule := range frontend.Rules {
			if offset >= len(resp.Rules.Rule) {
				needUpdate = true
				break
			}
			if resp.Rules.Rule[idx].RuleName != sc.getRuleName(rule) {
				needUpdate = true
				break
			}
			offset++
		}
		if offset != len(resp.Rules.Rule) || needUpdate {
			aliRules := resp.Rules.Rule[offset:]
			toDel := make([]string, 0, len(aliRules))
			for _, r := range aliRules {
				toDel = append(toDel, r.RuleId)
			}

			glog.Infof("Delete %d rules", len(toDel))
			for len(toDel) > 0 {
				i := 10
				if i > len(toDel) {
					i = len(toDel)
				}
				t := toDel[:i]
				delList, err := json.Marshal(t)
				if err != nil {
					glog.Error(err)
					return err
				}
				err = sc.SlbClient.DeleteRules(&slb.DeleteRulesArgs{
					RegionId: common.Region(sc.RegionName),
					RuleIds:  string(delList),
				})
				if err != nil {
					glog.Error(err)
					return err
				}
				toDel = toDel[i:]
			}

			type aliRule struct {
				RuleName       string
				Domain         string `json:",omitempty"`
				Url            string `json:",omitempty"`
				VServerGroupId string
			}
			// 10 is the max size per request
			rules := make([]aliRule, 0, 10)
			glog.Infof("Add %d rules.", len(frontend.Rules[offset:]))
			for _, rule := range frontend.Rules[offset:] {
				if len(rule.Domain) > 80 || len(rule.URL) > 80 {
					glog.Warningf("rule is too long to set. domain is %s, url is %s.", rule.Domain, rule.URL)
					continue
				}
				if len(rule.URL) > 0 && !strings.HasPrefix(rule.URL, "/") {
					glog.Warning("Illegal url ", rule.URL)
					continue
				}
				if rule.URL == "/" {
					rule.URL = ""
				}
				rules = append(
					rules,
					aliRule{
						RuleName:       sc.getRuleName(rule),
						Domain:         rule.Domain,
						Url:            rule.URL,
						VServerGroupId: sc.name2ID[rule.BackendGroup.Name],
					},
				)
				if len(rules) == 10 {
					ruleList, _ := json.Marshal(rules)
					glog.Infof("Create rules on LB %s port %d: %s.", loadBalancerID, frontend.Port, ruleList)
					err := sc.SlbClient.CreateRules(&slb.CreateRulesArgs{
						RegionId:       common.Region(sc.RegionName),
						LoadBalancerId: loadBalancerID,
						ListenerPort:   frontend.Port,
						RuleList:       string(ruleList),
					})
					if err != nil {
						glog.Error(err)
						return err
					}
					rules = make([]aliRule, 0, 10)
				}
			}
			if len(rules) > 0 {
				ruleList, _ := json.Marshal(rules)
				glog.Infof("Create rules on LB %s port %d: %s.", loadBalancerID, frontend.Port, ruleList)
				err := sc.SlbClient.CreateRules(&slb.CreateRulesArgs{
					RegionId:       common.Region(sc.RegionName),
					LoadBalancerId: loadBalancerID,
					ListenerPort:   frontend.Port,
					RuleList:       string(ruleList),
				})
				if err != nil {
					glog.Error(err)
					return err
				}
			}
		} else {
			glog.Info("Rules no change.")
		}
	}

	//handle default rule
	glog.Infof("check default rule of listener on port %d.", frontend.Port)
	desc, err := sc.describeLoadBalancerListener(loadBalancerID, frontend.Port, frontend.Protocol)
	if err != nil {
		return err
	}

	certificateID := ""
	if frontend.Protocol == ProtocolHTTPS {
		certificateID = frontend.CertificateID
	}
	if frontend.BackendGroup != nil {
		name := frontend.BackendGroup.Name
		vid := sc.name2ID[name]
		if vid != desc.VServerGroupID {
			glog.Infof("Update default rule to vsg %s:%s.", name, vid)
			err = sc.setListenerVGroupServer(loadBalancerID, frontend.Port, frontend.Protocol, vid, certificateID)
		}
	} else {
		if desc.VServerGroupID != "" {
			glog.Infof("unset default rule %s", desc.VServerGroupID)
			err = sc.setListenerVGroupServer(loadBalancerID, frontend.Port, frontend.Protocol, "", certificateID)
		}
	}
	if err != nil {
		glog.Error(err)
		return err
	}
	if desc.Status == slb.Stopped || desc.Status == slb.Stopping {
		glog.Infof("start listener on port %d", frontend.Port)
		if err := sc.SlbClient.StartLoadBalancerListener(loadBalancerID, frontend.Port); err != nil {
			glog.Error(err)
		}
	}
	return nil
}

func (sc *SlbController) reloadV2(lb *LoadBalancer) error {
	glog.Infof("Reload v2 slb %s.", lb.Name)
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(2)
	var vsgError, listenerError error
	go func() {
		defer wg.Done()
		vsgError = sc.updateVServerGroup(lb)
	}()
	go func() {
		defer wg.Done()
		listenerError = sc.updateListenerV2(lb)
	}()
	wg.Wait()
	if vsgError != nil {
		return vsgError
	}
	if listenerError != nil {
		return listenerError
	}

	cfg := generateConfig(lb)
	wg.Add(len(cfg.Frontends))
	ret := make(chan error, len(cfg.Frontends))

	for _, frontend := range cfg.Frontends {
		go func(ft *Frontend, ret chan<- error) {
			defer wg.Done()
			err := sc.updateRules(lb.LoadBalancerID, ft)
			ret <- err
		}(frontend, ret)
	}

	go func() {
		defer close(ret)
		wg.Wait()
	}()

	for {
		err, ok := <-ret
		if err != nil {
			return err
		}
		if !ok {
			// chan is closed.
			break
		}
	}

	// Delete useless vServerGroups
	for _, vid := range sc.vsgToDelete {
		if _, err := sc.SlbClient.DeleteVServerGroup(
			&slb.DeleteVServerGroupArgs{
				VServerGroupId: vid,
				RegionId:       common.Region(sc.RegionName),
			}); err != nil {
			glog.Error(err)
			return err
		}
	}

	duration := time.Now().Sub(start)
	glog.Infof("Reload %s used %.3f seconds.", lb.Name, float64(duration)/float64(time.Second))
	return nil
}

func (sc *SlbController) ReloadLoadBalancer() error {
	configStr := ""
	for _, lb := range sc.LoadBalancers {
		configStr = configStr + lb.String()
	}
	if configStr != LastConfig {
		glog.Infof("new config is %s", configStr)
		glog.Infof("old config is %s", LastConfig)
	}

	if configStr == LastConfig && !LastFailure {
		glog.Info("Config not changed")
		if time.Now().Sub(lastCheckTime) < 1*time.Minute {
			return nil
		}
		glog.Info("Re-sync config after a long while.")
	}
	LastConfig = configStr
	lastCheckTime = time.Now()

	var failure = false
	for _, lb := range sc.LoadBalancers {
		if lb.Version == 2 {
			if err := sc.reloadV2(lb); err != nil {
				failure = true
			}
			continue
		}
		output, err := sc.SlbClient.DescribeLoadBalancerAttribute(lb.LoadBalancerID)
		if err != nil {
			glog.Error(err.Error())
			glog.Error(lb)
			failure = true
			continue
		}

		currentListeners := []DescribeListenerAttribute{}
		for _, lt := range output.ListenerPortsAndProtocol.ListenerPortAndProtocol {
			res, err := sc.describeLoadBalancerListener(lb.LoadBalancerID, lt.ListenerPort, lt.ListenerProtocol)
			if err != nil {
				glog.Error(err.Error())
				failure = true
				continue
			}
			currentListeners = append(currentListeners, res)
		}

		// Add new listeners
		for _, ft := range lb.Frontends {
			backends := getBackends(ft)
			if len(backends) == 0 {
				continue
			}
			exist := false
			for _, cl := range currentListeners {
				if cl.ListenerPort == ft.Port {
					if backends[0].Port != cl.BackendPort {
						if err := sc.SlbClient.DeleteLoadBalancerListener(lb.LoadBalancerID, cl.ListenerPort); err != nil {
							glog.Errorf("Delete listeners failed %s", err.Error())
						}
					} else {
						exist = true
					}
					break
				}
			}

			if !exist {
				instancePort := backends[0].Port
				{
					if err := sc.createLoadBalancerListener(lb.LoadBalancerID, ft.Port, instancePort, ft.Protocol, ""); err != nil {
						glog.Errorf("Create listeners failed %s", err.Error())
					}
					if err := sc.SlbClient.SetLoadBalancerStatus(lb.LoadBalancerID, slb.ActiveStatus); err != nil {
						glog.Errorf("Set loadbalancer to active failed %s", err.Error())
					}
				}
			}
		}

		// Delete old listeners
		for _, cl := range currentListeners {
			if strings.ToLower(cl.ListenerProtocol) != ProtocolTCP &&
				strings.ToLower(cl.ListenerProtocol) != ProtocolHTTP {
				//Ignore unsupported listeners
				continue
			}
			shouldRemove := true
			for _, ft := range lb.Frontends {
				if ft.Port == cl.ListenerPort {
					shouldRemove = false
					break
				}
			}
			if shouldRemove {
				if err := sc.SlbClient.DeleteLoadBalancerListener(lb.LoadBalancerID, cl.ListenerPort); err != nil {
					glog.Errorf("Delete listeners failed %s", err.Error())
					failure = true
				}
			}
		}

		// Add new instances
		currentInstances := []string{}
		for _, i := range output.BackendServers.BackendServer {
			currentInstances = append(currentInstances, i.ServerId)
		}
		exceptIPs := []string{}
		if len(lb.Frontends) > 0 {
			for _, i := range getBackends(lb.Frontends[0]) {
				exceptIPs = append(exceptIPs, i.Address)
			}
		}

		exceptInstances, err := sc.getIdFromIP(exceptIPs)
		if err != nil {
			continue
		}

		addInstances := []slb.BackendServerType{}
		for _, i := range exceptInstances {
			if !strIn(&i, currentInstances) {
				addInstances = append(addInstances, slb.BackendServerType{ServerId: i, Weight: 100})
			}
		}
		if len(addInstances) > 0 {
			glog.Infof("LB %s add backend server %v", lb.Name, exceptInstances)
			_, err := sc.SlbClient.AddBackendServers(lb.LoadBalancerID, addInstances)
			if err != nil {
				glog.Error(err.Error())
				failure = true
			}
		}

		// Delete old instances
		removeInstances := []string{}
		for _, i := range currentInstances {
			if !strIn(&i, exceptInstances) {
				removeInstances = append(removeInstances, i)
			}
		}
		if len(removeInstances) > 0 {
			_, err := sc.SlbClient.RemoveBackendServers(lb.LoadBalancerID, removeInstances)
			if err != nil {
				glog.Error(err.Error())
				failure = true
			}
		}
	}
	LastFailure = failure
	return nil
}

func (sc *SlbController) createLoadBalancerListener(
	loadBalancerID string,
	listenerPort, backendServerPort int,
	listenerProtocol, certificateID string) (err error) {
	switch strings.ToUpper(listenerProtocol) {
	case "HTTPS":
		err = sc.SlbClient.CreateLoadBalancerHTTPSListener(&slb.CreateLoadBalancerHTTPSListenerArgs{
			HTTPListenerType: slb.HTTPListenerType{
				LoadBalancerId:         loadBalancerID,
				ListenerPort:           listenerPort,
				BackendServerPort:      backendServerPort,
				Bandwidth:              -1,
				StickySession:          slb.OffFlag,
				HealthCheck:            slb.OnFlag,
				HealthCheckConnectPort: -520,
				HealthCheckDomain:      "",
				HealthyThreshold:       3,
				UnhealthyThreshold:     3,
				HealthCheckTimeout:     3,
				HealthCheckInterval:    5,
				HealthCheckURI:         "/",
				HealthCheckHttpCode:    "http_2xx,http_3xx,http_4xx",
			},
			ServerCertificateId: certificateID,
		})
		return err
	case "HTTP":
		err = sc.SlbClient.CreateLoadBalancerHTTPListener(&slb.CreateLoadBalancerHTTPListenerArgs{
			LoadBalancerId:         loadBalancerID,
			ListenerPort:           listenerPort,
			BackendServerPort:      backendServerPort,
			Bandwidth:              -1,
			StickySession:          slb.OffFlag,
			HealthCheck:            slb.OnFlag,
			HealthCheckConnectPort: -520,
			HealthCheckDomain:      "",
			HealthyThreshold:       3,
			UnhealthyThreshold:     3,
			HealthCheckTimeout:     3,
			HealthCheckInterval:    5,
			HealthCheckURI:         "/",
			HealthCheckHttpCode:    "http_2xx,http_3xx,http_4xx",
		})
		return err
	case "TCP":
		err = sc.SlbClient.CreateLoadBalancerTCPListener(&slb.CreateLoadBalancerTCPListenerArgs{
			LoadBalancerId:    loadBalancerID,
			ListenerPort:      listenerPort,
			BackendServerPort: backendServerPort,
			Bandwidth:         -1,
		})
		return err
	case "UDP":
		err = sc.SlbClient.CreateLoadBalancerUDPListener(&slb.CreateLoadBalancerUDPListenerArgs{
			LoadBalancerId:    loadBalancerID,
			ListenerPort:      listenerPort,
			BackendServerPort: backendServerPort,
			Bandwidth:         -1,
		})
		return err
	}
	return errors.New(fmt.Sprintf("Protocol %s is not supported by aliyun.", listenerProtocol))

}

func (sc *SlbController) describeLoadBalancerListener(loadBalancerId string, listenerPort int, listenerProtocol string) (DescribeListenerAttribute, error) {
	switch strings.ToUpper(listenerProtocol) {
	case "HTTPS":
		res, err := sc.SlbClient.DescribeLoadBalancerHTTPSListenerAttribute(loadBalancerId, listenerPort)
		if err != nil {
			glog.Error(err.Error())
			return DescribeListenerAttribute{}, err
		}
		return DescribeListenerAttribute{
			Status:           res.Status,
			ListenerPort:     listenerPort,
			ListenerProtocol: listenerProtocol,
			BackendPort:      res.BackendServerPort,
			VServerGroupID:   res.VServerGroupId,
			CertificateID:    res.ServerCertificateId,
		}, nil
	case "HTTP":
		res, err := sc.SlbClient.DescribeLoadBalancerHTTPListenerAttribute(loadBalancerId, listenerPort)
		if err != nil {
			glog.Error(err.Error())
			return DescribeListenerAttribute{}, err
		}
		return DescribeListenerAttribute{
			Status:           res.Status,
			ListenerPort:     listenerPort,
			ListenerProtocol: listenerProtocol,
			BackendPort:      res.BackendServerPort,
			VServerGroupID:   res.VServerGroupId,
		}, nil
	case "TCP":
		res, err := sc.SlbClient.DescribeLoadBalancerTCPListenerAttribute(loadBalancerId, listenerPort)
		if err != nil {
			glog.Error(err.Error())
			return DescribeListenerAttribute{}, err
		}
		return DescribeListenerAttribute{
			Status:           res.Status,
			ListenerPort:     listenerPort,
			ListenerProtocol: listenerProtocol,
			BackendPort:      res.BackendServerPort,
			VServerGroupID:   res.VServerGroupId,
		}, nil
	case "UDP":
		res, err := sc.SlbClient.DescribeLoadBalancerUDPListenerAttribute(loadBalancerId, listenerPort)
		if err != nil {
			glog.Error(err.Error())
			return DescribeListenerAttribute{}, err
		}
		return DescribeListenerAttribute{
			Status:           res.Status,
			ListenerPort:     listenerPort,
			ListenerProtocol: listenerProtocol,
			BackendPort:      res.BackendServerPort,
			VServerGroupID:   res.VServerGroupId,
		}, nil
	}
	return DescribeListenerAttribute{}, errors.New(fmt.Sprintf("Protocol %s is not supported by aliyun.", listenerProtocol))
}

func (sc *SlbController) getIdFromIP(ip []string) ([]string, error) {
	res := []string{}
	if len(ip) == 0 {
		return res, nil
	}
	ips := "["
	for _, i := range ip {
		ips += "\"" + i + "\","
	}
	ips += "]"
	args := &ecs.DescribeInstancesArgs{
		RegionId:           common.Region(sc.RegionName),
		PrivateIpAddresses: ips,
	}
	args.PageSize = 100
	instances, _, err := sc.EcsClient.DescribeInstances(args)
	if len(instances) == 0 {
		args = &ecs.DescribeInstancesArgs{
			RegionId:         common.Region(sc.RegionName),
			InnerIpAddresses: ips,
		}
		args.PageSize = 100
		instances, _, _ = sc.EcsClient.DescribeInstances(args)
	}
	if err != nil {
		glog.Error(err.Error())
		return res, err
	}
	for _, r := range instances {
		res = append(res, r.InstanceId)
	}
	glog.Infof("query IP %s, get ID %s.", ip, res)
	return res, nil
}

func (sc *SlbController) setListenerVGroupServer(loadBalancerID string,
	port int, protocol, vServerGroupID, certificateID string) (err error) {

	switch strings.ToUpper(protocol) {
	case "HTTPS":
		err = sc.SlbClient.SetLoadBalancerHTTPSListenerAttribute(&slb.SetLoadBalancerHTTPSListenerAttributeArgs{
			HTTPListenerType: slb.HTTPListenerType{
				LoadBalancerId:         loadBalancerID,
				ListenerPort:           port,
				BackendServerPort:      InvalidBackendPort,
				Bandwidth:              -1,
				StickySession:          slb.OffFlag,
				HealthCheck:            slb.OnFlag,
				HealthCheckConnectPort: -520,
				HealthCheckDomain:      "",
				HealthyThreshold:       3,
				UnhealthyThreshold:     3,
				HealthCheckTimeout:     3,
				HealthCheckInterval:    5,
				HealthCheckURI:         "/",
				HealthCheckHttpCode:    "http_2xx,http_3xx,http_4xx",
				VServerGroup:           slb.OnFlag,
				VServerGroupId:         vServerGroupID,
			},
			ServerCertificateId: certificateID,
		})
	case "HTTP":
		err = sc.SlbClient.SetLoadBalancerHTTPListenerAttribute(&slb.SetLoadBalancerHTTPListenerAttributeArgs{
			LoadBalancerId:         loadBalancerID,
			ListenerPort:           port,
			BackendServerPort:      InvalidBackendPort,
			Bandwidth:              -1,
			StickySession:          slb.OffFlag,
			HealthCheck:            slb.OnFlag,
			HealthCheckConnectPort: -520,
			HealthCheckDomain:      "",
			HealthyThreshold:       3,
			UnhealthyThreshold:     3,
			HealthCheckTimeout:     3,
			HealthCheckInterval:    5,
			HealthCheckURI:         "/",
			HealthCheckHttpCode:    "http_2xx,http_3xx,http_4xx",
			VServerGroup:           slb.OnFlag,
			VServerGroupId:         vServerGroupID,
		})
	case "TCP":
		err = sc.SlbClient.SetLoadBalancerTCPListenerAttribute(&slb.SetLoadBalancerTCPListenerAttributeArgs{
			LoadBalancerId:    loadBalancerID,
			ListenerPort:      port,
			BackendServerPort: InvalidBackendPort,
			Bandwidth:         -1,
			VServerGroup:      slb.OnFlag,
			VServerGroupId:    vServerGroupID,
		})
	case "UDP":
		err = sc.SlbClient.SetLoadBalancerUDPListenerAttribute(&slb.SetLoadBalancerUDPListenerAttributeArgs{
			LoadBalancerId:    loadBalancerID,
			ListenerPort:      port,
			BackendServerPort: InvalidBackendPort,
			Bandwidth:         -1,
			VServerGroup:      slb.OnFlag,
			VServerGroupId:    vServerGroupID,
		})
	default:
		err = fmt.Errorf("un-supported protocol %s", protocol)
	}
	if err != nil {
		glog.Error(err)
	}
	return err
}
