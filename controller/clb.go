package controller

import (
	"alauda_lb/config"
	"alauda_lb/driver"
	"alauda_lb/util"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"bitbucket.org/mathildetech/qcloud-sdk-go/clb"
	"bitbucket.org/mathildetech/qcloud-sdk-go/common"
	"bitbucket.org/mathildetech/qcloud-sdk-go/cvm"
	"github.com/golang/glog"
)

const REGEXRULE = "~* "
const CLB_POLICY_WRR = "wrr"
const CLB_POLICY_HASH = "ip_hash"

type ClbController struct {
	RegionName         string
	Credential         common.Credential
	ClbClient          *clb.Client
	CvmClient          *cvm.Client
	Driver             driver.Driver
	LoadBalancers      []*LoadBalancer
	ip2InstanceIdCache map[string]string
}

type DescribeCLbListenerAttribute struct {
	ListenerPort     int
	ListenerProtocol string
	BackendPort      int
	VServerGroupID   string
	CertificateID    string
	Status           string
}

func (cc *ClbController) init() {
	cc.RegionName = config.Get("IAAS_REGION")
	cc.Credential = common.Credential{
		SecretId:  config.Get("ACCESS_KEY"),
		SecretKey: config.Get("SECRET_ACCESS_KEY"),
	}
	opts := common.Opts{
		Region: cc.RegionName,
	}
	cc.ClbClient, _ = clb.NewClient(cc.Credential, opts)
	cc.CvmClient, _ = cvm.NewClient(cc.Credential, opts)

	cc.ip2InstanceIdCache = make(map[string]string)
}

func (cc *ClbController) GetLoadBalancerType() string {
	return "clb"
}

func (cc *ClbController) GenerateConf() error {
	loadbalancers, err := FetchLoadBalancersInfo()
	if err != nil {
		return err
	}
	loadbalancers = filterLoadbalancers(loadbalancers, "clb", "")
	if err != nil {
		return err
	}
	services, err := cc.Driver.ListService()
	if err != nil {
		return err
	}
	merge(loadbalancers, services)
	cc.LoadBalancers = loadbalancers
	return nil
}

func (cc *ClbController) ReloadLoadBalancer() error {
	configStr := ""
	for _, lb := range cc.LoadBalancers {
		configStr = configStr + lb.String()
	}
	glog.Infof("new config is %s", configStr)
	glog.Infof("old config is %s", LastConfig)
	if configStr == LastConfig && !LastFailure {
		glog.Info("clb Config not changed")
		if time.Now().Sub(lastCheckTime) < 1*time.Minute {
			return nil
		}
		glog.Info("Re-sync config after a long while.")
	}
	LastConfig = configStr
	lastCheckTime = time.Now()

	failure := false
	for _, lb := range cc.LoadBalancers {
		if lb.Version == 2 {
			if err := cc.reloadV2(lb); err != nil {
				failure = true
			}
			continue
		} else {
			glog.Errorln("wrong clb version.")
		}
	}
	LastFailure = failure
	return nil
}

func (cc *ClbController) reloadV2(lb *LoadBalancer) error {
	glog.Infof("Reload v2 clb %s.", lb.Name)
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(1)
	var listenerError error
	go func() {
		defer wg.Done()
		listenerError = cc.updateListenerV2(lb)
	}()
	wg.Wait()
	if listenerError != nil {
		return listenerError
	}
	wg.Add(len(lb.Frontends))
	ret := make(chan error, len(lb.Frontends))

	for _, frontend := range lb.Frontends {
		go func(ft *Frontend, ret chan<- error) {
			defer wg.Done()
			err := cc.updateRules(lb.LoadBalancerID, ft)
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

	duration := time.Now().Sub(start)
	glog.Infof("Reload %s used %.3f seconds.", lb.Name, float64(duration)/float64(time.Second))
	return nil
}

func (cc *ClbController) updateListenerV2(lb *LoadBalancer) error {
	glog.Info("begin update ListenerV2")
	config := generateConfig(lb)
	descListenerArg := &clb.DescribeForwardLBListenersArgs{
		LoadBalancerId: lb.LoadBalancerID,
	}
	listenerAttrs, err := cc.ClbClient.DescribeForwardLoadBalancerListeners(descListenerArg)
	if err != nil {
		glog.Error(err)
		return err
	}
	delListenerIds := []string{}

	for _, listenerSet := range listenerAttrs.ListenerSet {
		glog.Infof("range clb listener port on %d", listenerSet.LoadBalancerPort)
		frontend, ok := config.Frontends[listenerSet.LoadBalancerPort]
		if ok && frontend.Protocol == listenerSet.ProtocolType {
			frontend.ready = true
		} else {
			delListenerIds = append(delListenerIds, listenerSet.ListenerId)
			glog.Infof("add listener port %d on LB %s to delListenerIds array", listenerSet.LoadBalancerPort, lb.Name)
		}
		if ok && frontend.Protocol == ProtocolHTTPS {
			// for an https listener, cerfificated id should be compared.
			if listenerSet.CertId != frontend.CertificateID {
				frontend.ready = false
				delListenerIds = append(delListenerIds, listenerSet.ListenerId)
				glog.Infof("add listener port %d on LB %s to delListenerIds array because CertId different", listenerSet.LoadBalancerPort, lb.Name)
			}
		}
	}
	if len(delListenerIds) > 0 {
		glog.Infof("delListenerIds array is %s", strings.Join(delListenerIds, ","))
		// delete listener
		for _, listenerId := range delListenerIds {
			glog.Infof("delete listener %s", listenerId)
			if _, err := cc.ClbClient.DeleteLoadBalancerListeners(lb.LoadBalancerID, listenerId); err != nil {
				glog.Error(err)
				continue
			}
		}
		//todo synctask
	}
	// storage the new listeners infos
	fourLayerLbListenerOpts := []clb.CreateFourLayerListenerOpts{}
	sevenLayerLbListenerOpts := []clb.CreateSevenLayerListenerOpts{}
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
		if f.Protocol == ProtocolTCP || f.Protocol == ProtocolUDP {
			newListener := clb.CreateFourLayerListenerOpts{
				LoadBalancerPort: f.Port,
				Protocol:         cc.getProtocolCode(f.Protocol),
				ListenerName:     util.RandomStr(f.Protocol, 8),
				SessionExpire:    0,
				HealthSwitch:     1,
				TimeOut:          2,
				IntervalTime:     5,
				HealthNum:        3,
				UnhealthNum:      3,
			}
			fourLayerLbListenerOpts = append(fourLayerLbListenerOpts, newListener)
		}
		if f.Protocol == ProtocolHTTPS || f.Protocol == ProtocolHTTP {
			newListener := clb.CreateSevenLayerListenerOpts{
				LoadBalancerPort: f.Port,
				Protocol:         cc.getProtocolCode(f.Protocol),
				CertId:           certificateID,
				ListenerName:     util.RandomStr(f.Protocol, 8),
			}
			sevenLayerLbListenerOpts = append(sevenLayerLbListenerOpts, newListener)
		}
	}
	// create listener
	if len(fourLayerLbListenerOpts) > 0 {
		if err := cc.createFourLayerLoadBalancerListener(lb.LoadBalancerID, fourLayerLbListenerOpts); err != nil {
			glog.Error(err)
			return err
		}
	}
	if len(sevenLayerLbListenerOpts) > 0 {
		if err := cc.createSevenLayerLoadBalancerListener(lb.LoadBalancerID, sevenLayerLbListenerOpts); err != nil {
			glog.Error(err)
			return err
		}
	}
	return nil
}

/**
change protocol string to protocol code by clb recognized
*/
func (cc *ClbController) getProtocolCode(protocol string) int {
	switch strings.ToLower(protocol) {
	case ProtocolHTTP:
		return clb.LoadBalanceListenerProtocolHTTP
	case ProtocolHTTPS:
		return clb.LoadBalanceListenerProtocolHTTPS
	case ProtocolTCP:
		return clb.LoadBalanceListenerProtocolTCP
	case ProtocolUDP:
		return clb.LoadBalanceListenerProtocolUDP
	default:
		return clb.LoadBalanceListenerProtocolHTTP
	}
}

func (cc *ClbController) createFourLayerLoadBalancerListener(loadBalancerID string, four []clb.CreateFourLayerListenerOpts) (err error) {
	fourArgs := &clb.CreateFourLayerLoadBalancerListenersArgs{
		LoadBalancerId: loadBalancerID,
		Listeners:      four,
	}
	resFour, err := cc.ClbClient.CreateFourLayerLoadBalancerListeners(fourArgs)
	glog.Errorln(resFour)
	return err
}

func (cc *ClbController) createSevenLayerLoadBalancerListener(loadBalancerID string, seven []clb.CreateSevenLayerListenerOpts) (err error) {
	sevenArgs := &clb.CreateSevenLayerLoadBalancerListenersArgs{
		LoadBalancerId: loadBalancerID,
		Listeners:      seven,
	}
	resSeven, err := cc.ClbClient.CreateSevenLayerLoadBalancerListeners(sevenArgs)
	glog.Errorln(resSeven)
	return err
}

func (cc *ClbController) getRuleNameKey(domain, url string, LbPort int) string {
	h := md5.New()
	io.WriteString(h, domain)
	io.WriteString(h, url)
	io.WriteString(h, fmt.Sprint(LbPort))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (cc *ClbController) updateRules(loadBalancerID string, frontend *Frontend) error {
	if len(frontend.Rules) == 0 && frontend.BackendGroup == nil {
		glog.Errorf("skip updateRules for invalid frontend for %s on port %d because rules and backend is nil", loadBalancerID, frontend.Port)
		return nil
	}
	if frontend.Port == 0 {
		glog.Errorf("Lb %s updateRules error, because frontend's port is 0", loadBalancerID)
		return errors.New(fmt.Sprintf("Lb %s updateRules err, because frontend's port is 0", loadBalancerID))
	}
	if frontend.Protocol == ProtocolHTTP || frontend.Protocol == ProtocolHTTPS {
		resp, err := cc.ClbClient.DescribeForwardLoadBalancerListeners(
			&clb.DescribeForwardLBListenersArgs{
				LoadBalancerId: loadBalancerID,
			})
		if err != nil {
			glog.Error(err)
			return err
		}
		if len(resp.ListenerSet) == 0 {
			glog.Errorf("updateRules error,lb %s on port %d has no listeners, listener may be not created!", loadBalancerID, frontend.Port)
			return errors.New(fmt.Sprintf("updateRules error,lb %s on port %d has no listeners!", loadBalancerID, frontend.Port))
		}

		currentListener := clb.Listener{}
		for _, tlistener := range resp.ListenerSet {
			if tlistener.LoadBalancerPort == frontend.Port {
				currentListener = tlistener
				break
			}
		}
		if currentListener.LoadBalancerPort != frontend.Port {
			glog.Errorf("updateRules error,lb %s on port %d has no listeners!", loadBalancerID, frontend.Port)
			return errors.New(fmt.Sprintf("updateRules error,lb %s on port %d has no listeners!", loadBalancerID, frontend.Port))
		}
		toAdd := []*Rule{}
		toUpdate := []*Rule{}
		ruleMap := make(map[string]string)
		rulePolicyMap := make(map[string]string)
		clbLoadBalancerInfo, err := json.Marshal(resp)
		glog.Infof("DescribeForwardLoadBalancerListeners LoadBalancerId %s,LoadBalancerPort %d", loadBalancerID, frontend.Port)
		glog.Infoln(string(clbLoadBalancerInfo))
		for _, rule := range currentListener.Rules {
			rname := cc.getRuleNameKey(rule.Domain, rule.URL, currentListener.LoadBalancerPort)
			glog.Infof("clb rule name ==== %s", rname)
			ruleMap[rname] = rule.LocationId
			rulePolicyMap[rname] = rule.HttpHash
		}

		// handle default service
		if frontend.BackendGroup != nil {
			//
			needAppendDefaultServiceRule := []*Rule{}
			defaultServiceDomain := make(map[string]bool)
			for _, rule := range frontend.Rules {
				if defaultServiceDomain[rule.Domain] {
					continue
				}
				if rule.URL == "" || rule.URL == "/" {
					rule.URL = "/"
					defaultServiceDomain[rule.Domain] = true
					continue
				} else {
					defaultServiceDomain[rule.Domain] = false
				}
			}
			//get need append default service rules
			for _, rule := range frontend.Rules {
				if !defaultServiceDomain[rule.Domain] {
					needAppendDefaultServiceRule = append(needAppendDefaultServiceRule, rule)
				}
			}
			for _, noDefaultServiceRule := range needAppendDefaultServiceRule {
				defaultServiceRule := &Rule{
					Domain: noDefaultServiceRule.Domain,
					URL:    "/",
					//CertificateID:frontend.CertificateID,
					BackendGroup: frontend.BackendGroup,
				}
				glog.Infof("add default service for domain %s ", noDefaultServiceRule.Domain)
				frontend.Rules = append(frontend.Rules, defaultServiceRule)
			}

		}
		policyExChange := map[string]string{
			PolicySIPHash: CLB_POLICY_HASH,
			PolicyCookie:  CLB_POLICY_WRR,
		}
		//general handle rules and backends
		for _, rule := range frontend.Rules {
			if len(rule.URL) > 0 && strings.HasPrefix(rule.URL, "^") {
				rule.URL = strings.Replace(rule.URL, "^", "", 1)
				rule.URL = REGEXRULE + rule.URL
			}
			if rule.URL == "" {
				rule.URL = "/"
			}
			name := cc.getRuleNameKey(rule.Domain, rule.URL, frontend.Port)
			glog.Infof("alauda rule name ==== %s", name)
			if _, ok := ruleMap[name]; ok {
				toUpdate = append(toUpdate, rule)
				if rulePolicyMap[name] != policyExChange[rule.SessionAffinityPolicy] {
					//todo
					glog.Infof("update rule sessionPolicy for lb %s location %s", loadBalancerID, ruleMap[name])
					clbHttpHash, clbSessionExpire := cc.getPolicyInfo(rule.SessionAffinityPolicy)
					args := &clb.ModifyLoadBalancerRulesProbeArgs{
						LoadBalancerId: loadBalancerID,
						LocationId:     ruleMap[name],
						ListenerId:     currentListener.ListenerId,
						HttpHash:       clbHttpHash,
						SessionExpire:  clbSessionExpire,
					}
					err := cc.modifyLoadBalancerRulesProbeArgs(args)
					if err != nil {
						glog.Error(err)
						continue
					}
				}
				delete(ruleMap, name)
			} else {
				toAdd = append(toAdd, rule)
			}
		}
		// handle delete rule
		if len(ruleMap) > 0 {
			toDel := make([]string, 0, len(ruleMap))
			for _, ruleID := range ruleMap {
				toDel = append(toDel, ruleID)
			}
			// delList, _ := json.Marshal(toDel)
			err := cc.ClbClient.DeleteRules(&clb.DeleteRulesArgs{
				LoadBalancerId: loadBalancerID,
				ListenerId:     currentListener.ListenerId,
				LocationIds:    &toDel,
			})
			if err != nil {
				glog.Error(err)
				return err
			}
		}
		// handle add rule
		if len(toAdd) > 0 {
			//add new rule
			toAddclbRuleList := []clb.CreateRuleArrRule{}
			for _, rule := range toAdd {
				if len(rule.Domain) > 80 || len(rule.URL) > 80 {
					glog.Warningf("rule is too long to set. domain is %s, url is %s.", rule.Domain, rule.URL)
					continue
				}
				/*if len(rule.URL) > 0 && strings.HasPrefix(rule.URL, "^"){
					rule.URL = strings.Replace(rule.URL, "^", "", 1)
					rule.URL = REGEXRULE + rule.URL
				}*/
				if rule.URL == "" {
					rule.URL = "/"
				}
				clbHttpHash, clbSessionExpire := cc.getPolicyInfo(rule.SessionAffinityPolicy)
				newRule := clb.CreateRuleArrRule{
					Domain:        rule.Domain,
					URL:           rule.URL,
					IntervalTime:  300,
					HealthNum:     3,
					UnhealthNum:   3,
					HttpCode:      31,
					HttpHash:      clbHttpHash,      //ip_hash wrr
					SessionExpire: clbSessionExpire, //based on cookie
				}
				toAddclbRuleList = append(toAddclbRuleList, newRule)
				//for update backends
				toUpdate = append(toUpdate, rule)
			}
			ruleList, _ := json.Marshal(toAddclbRuleList)
			glog.Infof("Create rules on LB %s port %d: %s.", loadBalancerID, frontend.Port, string(ruleList))
			if len(toAddclbRuleList) > 0 {
				cresp, cerr := cc.ClbClient.CreateRules(&clb.CreateRuleArgs{
					LoadBalancerId: loadBalancerID,
					ListenerId:     currentListener.ListenerId,
					Rules:          toAddclbRuleList,
				})
				if cerr != nil {
					glog.Error(cerr)
					return cerr
				}
				taskId := cresp.RequestId
				//todo  wait for rule create done
				if err = cc.waitForRuleDone(taskId); err == nil {
					// update Backends
					err = cc.updateSevenFloorBackends(loadBalancerID, frontend.Port, toUpdate)
					if err != nil {
						return err
					}
				}
			}
		} else {
			if len(toUpdate) > 0 {
				err = cc.updateSevenFloorBackends(loadBalancerID, frontend.Port, toUpdate)
				if err != nil {
					return err
				}
			}
		}

	} else if frontend.Protocol == ProtocolTCP || frontend.Protocol == ProtocolUDP {
		//update backends
		resp, err := cc.ClbClient.DescribeLoadBalancerBackends(
			&clb.DescribeLoadBalancerBackendsArgs{
				LoadBalancerId:   loadBalancerID,
				LoadBalancerPort: frontend.Port,
			})
		if err != nil {
			glog.Error(err)
			return err
		}
		for _, tlistener := range resp.Data {
			cc.updateFourFloorBackends(loadBalancerID, tlistener.ListenerId, frontend.BackendGroup.Backends, tlistener.Backends)
			break
		}
	} else {
		glog.Errorf("wrong protocol for Lb %s updateRules err", loadBalancerID)
		return errors.New(fmt.Sprintf("wrong protocol for Lb %s updateRules err", loadBalancerID))
	}
	return nil
}

func (cc *ClbController) getPolicyInfo(policy string) (httpHash string, expireTime int) {
	clbHttpHash := CLB_POLICY_WRR
	clbSessionExpire := 0
	if policy == PolicySIPHash {
		clbHttpHash = CLB_POLICY_HASH
	}
	if policy == PolicyCookie {
		clbSessionExpire = 3600
	}
	return clbHttpHash, clbSessionExpire
}

func (cc *ClbController) modifyLoadBalancerRulesProbeArgs(args *clb.ModifyLoadBalancerRulesProbeArgs) error {
	resp, err := cc.ClbClient.ModifyLoadBalancerRulesProbe(args)
	if err != nil {
		glog.Errorf("modifyLoadBalancerRulesProbeArgs err,%s", fmt.Sprint(err))
		return errors.New(fmt.Sprintf("modifyLoadBalancerRulesProbeArgs err, %s", fmt.Sprint(err)))
	}
	cc.waitForRuleDone(resp.RequestId)
	return nil
}

func (cc *ClbController) updateFourFloorBackends(LoadBalancerId string, ListenerId string, alaudaBackends []*Backend, clbBackends []clb.LoadBalancerBackends) error {
	toDelClbBackends := cc.getToDelBackends(alaudaBackends, clbBackends)
	if len(toDelClbBackends) > 0 {
		// delete backends on rule
		deRegisterArgs := &clb.DeregisterInstancesFromForwardLBFourthListenerArgs{
			LoadBalancerId: LoadBalancerId,
			ListenerId:     ListenerId,
			Backends:       toDelClbBackends,
		}
		cc.ClbClient.DeregisterInstancesFromForwardLBFourthListener(deRegisterArgs)
	}
	toAddClbBackends := cc.getToAddBackends(alaudaBackends, clbBackends)
	if len(toAddClbBackends) > 0 {
		// add backends on rule
		if len(toAddClbBackends) > 0 {
			// delete backends on rule
			registerArgs := &clb.RegisterInstancesWithForwardLBFourthListenerArgs{
				LoadBalancerId: LoadBalancerId,
				ListenerId:     ListenerId,
				Backends:       toAddClbBackends,
			}
			cc.ClbClient.RegisterInstancesWithForwardLBFourthListener(registerArgs)
		}
	}
	return nil
}

func (cc *ClbController) getSimpleRuleByDomainAndUrl(clbListener *clb.ListenerWithRules, LoadBalancerId string, LoadBalancerPort int, domain, url string) *clb.SimpleRule {
	res := &clb.SimpleRule{}
	for _, clbRule := range clbListener.Rules {
		if clbRule.URL == url && clbRule.Domain == domain {
			return &clbRule
		}
	}
	resp, err := cc.ClbClient.DescribeForwardLoadBalancerListeners(
		&clb.DescribeForwardLBListenersArgs{
			LoadBalancerId: LoadBalancerId,
		})
	if err != nil {
		glog.Error(err)
		return res
	}
	for _, tlistener := range resp.ListenerSet {
		if tlistener.LoadBalancerPort == LoadBalancerPort {
			for _, rule := range tlistener.Rules {
				if rule.Domain == domain && rule.URL == url {
					res.Domain = domain
					res.URL = url
					res.LocationId = rule.LocationId
				}
			}
			break
		}
	}
	return res
}

func (cc *ClbController) updateSevenFloorBackends(LoadBalancerId string, LoadBalancerPort int, rules []*Rule) error {
	//
	listenerAttrs, err := cc.ClbClient.DescribeLoadBalancerBackends(&clb.DescribeLoadBalancerBackendsArgs{
		LoadBalancerId:   LoadBalancerId,
		LoadBalancerPort: LoadBalancerPort,
	})
	if err != nil {
		glog.Errorf("CLB with LoadBalancerId %s LoadBalancerPort %d DescribeLoadBalancerBackends error", LoadBalancerId, LoadBalancerPort)
		return errors.New(fmt.Sprintf("CLB with LoadBalancerId %s LoadBalancerPort %d DescribeLoadBalancerBackends error", LoadBalancerId, LoadBalancerPort))
	}
	if len(listenerAttrs.Data) == 0 {
		glog.Errorf("updateSevenFloorBackends error, CLB with LoadBalancerId %s LoadBalancerPort %d has no listeners", LoadBalancerId, LoadBalancerPort)
		return errors.New(fmt.Sprintf("updateSevenFloorBackends error, CLB with LoadBalancerId %s LoadBalancerPort %d has no listeners", LoadBalancerId, LoadBalancerPort))
	}
	if len(listenerAttrs.Data) > 1 {
		glog.Errorf("CLB with LoadBalancerId %s LoadBalancerPort %d get two listeners", LoadBalancerId, LoadBalancerPort)
		return errors.New(fmt.Sprintf("CLB with LoadBalancerId %s LoadBalancerPort %d get two listeners", LoadBalancerId, LoadBalancerPort))
	}
	currentListener := listenerAttrs.Data[0]
	hasError := false
	errInfo := ""
	for _, rule := range rules {
		clbRule := cc.getSimpleRuleByDomainAndUrl(&currentListener, LoadBalancerId, LoadBalancerPort, rule.Domain, rule.URL)
		if clbRule.LocationId == "" {
			glog.Errorf("clbRule location for lb %s, rule %s, url %s is null", LoadBalancerId, rule.Domain, rule.URL)
			continue
		}
		glog.Infof("clbRule for lb %s, rule %s, url %s", LoadBalancerId, rule.Domain, rule.URL)
		terr, _ := json.Marshal(clbRule)
		glog.Infof(string(terr))
		//DeregisterInstancesFromForwardLB
		toDelClbBackends := cc.getToDelBackends(rule.BackendGroup.Backends, clbRule.Backends)
		if len(toDelClbBackends) > 0 {
			// delete backends on rule
			locationIds := []string{clbRule.LocationId}
			deRegisterArgs := &clb.DeregisterInstancesFromForwardLBArgs{
				LoadBalancerId: LoadBalancerId,
				ListenerId:     currentListener.ListenerId,
				Backends:       toDelClbBackends,
				LocationIds:    locationIds,
			}
			cc.ClbClient.DeregisterInstancesFromForwardLB(deRegisterArgs)
		}
		glog.Infof("getToAddBackends for lb %s, rule %s, url %s", LoadBalancerId, rule.Domain, rule.URL)
		//RegisterInstancesFromForwardLB
		toAddClbBackends := cc.getToAddBackends(rule.BackendGroup.Backends, clbRule.Backends)
		terrq, _ := json.Marshal(toAddClbBackends)
		glog.Infoln(string(terrq))
		if len(toAddClbBackends) > 0 {
			// add backends on rule
			if len(toAddClbBackends) > 0 {
				// delete backends on rule
				locationIds := []string{clbRule.LocationId}
				registerArgs := &clb.RegisterInstancesWithForwardLBSeventhListenerArgs{
					LoadBalancerId: LoadBalancerId,
					ListenerId:     currentListener.ListenerId,
					Backends:       toAddClbBackends,
					LocationIds:    locationIds,
				}
				_, err = cc.ClbClient.RegisterInstancesWithForwardLBSeventhListener(registerArgs)
				if err != nil {
					hasError = true
					errInfo = fmt.Sprint(err)
					glog.Errorf("RegisterInstancesWithForwardLBSeventhListener for lb %s , listener %s , rule %s error, %s",
						LoadBalancerId, currentListener.ListenerId, clbRule.LocationId, errInfo)
				}
			}
		}

	}
	if hasError {
		return errors.New(errInfo)
	}
	return nil
}

func (cc *ClbController) DescribeRuleAttributeByDomainAndUrl(LoadBalancerId string, LoadBalancerPort int, Domain, Url string) (*clb.ListenerWithRules, error) {
	listenerAttrs, err := cc.ClbClient.DescribeLoadBalancerBackends(&clb.DescribeLoadBalancerBackendsArgs{
		LoadBalancerId:   LoadBalancerId,
		LoadBalancerPort: LoadBalancerPort,
	})
	if err != nil {
		glog.Errorln("CLB with LoadBalancerId %s LoadBalancerPort %d DescribeLoadBalancerBackends error", LoadBalancerId, LoadBalancerPort)
		return &clb.ListenerWithRules{}, errors.New(fmt.Sprintf("CLB with LoadBalancerId %s LoadBalancerPort %d DescribeLoadBalancerBackends error", LoadBalancerId, LoadBalancerPort))
	}
	if len(listenerAttrs.Data) > 1 {
		glog.Errorln("CLB with LoadBalancerId %s LoadBalancerPort %d get two listeners", LoadBalancerId, LoadBalancerPort)
		return &clb.ListenerWithRules{}, errors.New(fmt.Sprintf("CLB with LoadBalancerId %s LoadBalancerPort %d get two listeners", LoadBalancerId, LoadBalancerPort))
	}
	result := listenerAttrs.Data[0]
	filterRule := []clb.SimpleRule{}
	for _, rule := range result.Rules {
		if rule.URL == Url && rule.Domain == Domain {
			filterRule = append(filterRule, rule)
		}
	}
	result.Rules = filterRule
	return &result, nil
}

func (cc *ClbController) getToDelBackends(alaudaBackends []*Backend, clbBackends []clb.LoadBalancerBackends) (res []clb.RegisterInstancesOpts) {
	result := []clb.RegisterInstancesOpts{}
	for _, clbBackend := range clbBackends {
		clbAddress := clbBackend.LanIp
		clbPort := clbBackend.Port
		clbWeight := clbBackend.Weight
		flag := false
		for _, alaudaBackend := range alaudaBackends {
			alaAddress := alaudaBackend.Address
			alaPort := alaudaBackend.Port
			albWeight := alaudaBackend.Weight
			if clbAddress == alaAddress && clbPort == alaPort && clbWeight == albWeight {
				flag = true
			}
		}
		if _, ok := cc.ip2InstanceIdCache[clbBackend.LanIp]; !ok {
			Instance, err := cc.getInstanceInfoByLanIp(clbBackend.LanIp)
			if err != nil {
				glog.Errorln("getInstanceInfoByLanIp error,LanIp is %s ", clbBackend.LanIp)
				continue
			}
			cc.ip2InstanceIdCache[clbBackend.LanIp] = Instance.UnInstanceId
		}
		newRegisterInstance := clb.RegisterInstancesOpts{
			Port:       clbBackend.Port,
			InstanceId: cc.ip2InstanceIdCache[clbBackend.LanIp],
		}
		if !flag {
			result = append(result, newRegisterInstance)
		}
	}
	return result
}

func (cc *ClbController) getToAddBackends(alaudaBackends []*Backend, clbBackends []clb.LoadBalancerBackends) (res []clb.RegisterInstancesOpts) {
	result := []clb.RegisterInstancesOpts{}
	for _, alaudaBackend := range alaudaBackends {
		alaAddress := alaudaBackend.Address
		alaPort := alaudaBackend.Port
		albWeight := alaudaBackend.Weight
		flag := true
		for _, clbBackend := range clbBackends {
			clbAddress := clbBackend.LanIp
			clbPort := clbBackend.Port
			clbWeight := clbBackend.Weight
			if clbAddress == alaAddress && clbPort == alaPort && clbWeight == albWeight {
				flag = false
			}
		}
		if flag {
			if _, ok := cc.ip2InstanceIdCache[alaudaBackend.Address]; !ok {
				Instance, err := cc.getInstanceInfoByLanIp(alaudaBackend.Address)
				if err != nil {
					glog.Errorln("getInstanceInfoByLanIp error,LanIp is %s ", alaudaBackend.Address)
					continue
				}
				cc.ip2InstanceIdCache[alaudaBackend.Address] = Instance.UnInstanceId
			}
			newRegisterInstance := clb.RegisterInstancesOpts{
				Port:       alaudaBackend.Port,
				InstanceId: cc.ip2InstanceIdCache[alaudaBackend.Address],
				Weight:     alaudaBackend.Weight,
			}
			result = append(result, newRegisterInstance)
		}
	}
	return result
}

func (cc *ClbController) getInstanceInfoByLanIp(lanIp string) (instance *cvm.InstanceType, err error) {
	resp, err := cc.CvmClient.DescribeInstances(&cvm.DescribeInstanceArgs{
		LanIp: lanIp,
	})
	if err != nil {
		return &cvm.InstanceType{}, err
	}
	return resp[0], nil
}

func (cc *ClbController) waitForRuleDone(taskId int) error {
	timeout := time.After(time.Duration(20) * time.Second)
	ch := make(chan int)
	go func(ch chan int) {
		defer close(ch)
		for {
			time.Sleep(2 * time.Second)
			glog.Infof("DescribeLoadBalancersTaskResult for %d ", taskId)
			tresp, terr := cc.ClbClient.DescribeLoadBalancersTaskResult(taskId)
			if terr != nil {
				glog.Error(terr)
				break
			}
			if tresp.Data.Status == 0 {
				ch <- tresp.Data.Status
			}
		}
	}(ch)
watchdog:
	for {
		glog.Infof("wait for rule create done task %d", taskId)
		select {
		case <-ch:
			//read data from ch
			glog.Infof("DescribeLoadBalancersTaskResult for %d  has done done ------", taskId)
			break watchdog
		case <-timeout:
			glog.Errorf("timeout for waitForRuleDone task with Id %d", taskId)
			return errors.New("timeout for waitForRuleDone task with Id " + fmt.Sprint(taskId))
		}
	}
	return nil
}
