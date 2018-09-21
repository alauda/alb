package clb

import (
	"github.com/golang/glog"
)

type Rule struct {
	LocationId		string		`json:"locationId"`
	Domain			string		`json:"domain"`
	URL				string		`json:"url"`
	HttpHash		string		`json:"httpHash"`
	SessionExpire	int			`json:"sessionExpire"`
	HealthSwitch	int			`json:"healthSwitch"`
	TimeOut			int			`json:"timeOut"`
	IntervalTime	int			`json:"intervalTime"`
	HealthNum		int			`json:"healthNum"`
	UnhealthNum		int			`json:"unhealthNum"`
	HttpCode		int			`json:"httpCode"`
	HttpCheckPath	string		`json:"httpCheckPath"`
}

type SimpleRule struct {
	LocationId  string `json:"locationId"`
	Domain      string `json:"domain"`
	URL         string `json:"url"`
	Backends    []LoadBalancerBackends	`json:"backends"`
}

type CreateRuleArgs struct {
	LoadBalancerId		string	`ArgName:"loadBalancerId"`
	ListenerId			string  `ArgName:"listenerId"`
	Rules				[]CreateRuleArrRule	`ArgName:"rules"`
}

type CreateRuleArrRule struct {
	Domain			string		`ArgName:"domain"`
	URL				string		`ArgName:"url"`
	HttpHash		string		`ArgName:"httpHash"`
	SessionExpire	int			`ArgName:"sessionExpire"`
	HealthSwitch	int			`ArgName:"healthSwitch"`
	IntervalTime	int			`ArgName:"intervalTime"`
	HealthNum		int			`ArgName:"healthNum"`
	UnhealthNum		int			`ArgName:"unhealthNum"`
	HttpCode		int			`ArgName:"httpCode"`
	HttpCheckPath	string		`ArgName:"httpCheckPath"`
}

type CreateRulesResponse struct {
	RequestId		int		`json:"requestId"`
}

func (client *Client) CreateRules(args *CreateRuleArgs) (resp *CreateRulesResponse, err error) {
	response := &CreateRulesResponse{}
	err = client.Invoke("CreateForwardLBListenerRules", args, &response)
	if err != nil {
		return nil, err
	}
	return response, err
}

type DeleteRulesArgs struct {
	LoadBalancerId		string	`ArgName:"loadBalancerId"`
	ListenerId			string  `ArgName:"listenerId"`
	LocationIds			*[]string `ArgName:"locationIds"`
}

type DeleteRulesResponse struct {
	RequestId		int		`json:"requestId"`
}

// Delete forward rules
//
func (client *Client) DeleteRules(args *DeleteRulesArgs) error {
	response := DeleteRulesResponse{}
	err := client.Invoke("DeleteForwardLBListenerRules", args, &response)
	if err != nil {
		return err
	}
	return err
}

type DescribeRuleAttributeArgs struct {
	LoadBalancerId string	`ArgName:"loadBalancerId"`
	LoadBalancerPort int	`ArgName:"loadBalancerPort"`
	Domain string	`ArgName:"domain"`
	Url string	`ArgName:"url"`
}

type DescribeRuleAttributeResponse struct {
	Data	[]Rule
}

// Describe rule
//
func (client *Client) DescribeRuleAttributeByDomainAndUrl(args *DescribeRuleAttributeArgs) ([]SimpleRule, error) {
	result := []SimpleRule{}
	listenerAttrs, err := client.DescribeLoadBalancerBackends(&DescribeLoadBalancerBackendsArgs{
		LoadBalancerId:	args.LoadBalancerId,
		LoadBalancerPort: args.LoadBalancerPort,
	})
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	for _, listenerSet := range listenerAttrs.Data {
		for _, rule := range listenerSet.Rules{
			addFlag := true
			if args.Domain != "" && rule.Domain != args.Domain{
				addFlag = false
			}
			if args.Url != "" && rule.URL != args.Url{
				addFlag = false
			}
			if addFlag {
				result = append(result, rule)
			}
		}
	}

	return result, nil
}

type DescribeRulesArgs struct {
	LoadBalancerId string
	ListenerPort   int
}

type DescribeRulesResponse struct {
	Rules struct {
		Rule []Rule
	}
}

// Describe rule
//
func (client *Client) DescribeRules(args *DescribeRulesArgs) (*DescribeRulesResponse, error) {
	response := &DescribeRulesResponse{}
	err := client.Invoke("DescribeRules", args, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

type ModifyLoadBalancerRulesProbeArgs struct {
	LoadBalancerId	string  `ArgName:"loadBalancerId"`
	ListenerId		string  `ArgName:"listenerId"`
	LocationId		string   `ArgName:"locationId"`
	Url 			string 	`ArgName:"url"`
	SessionExpire	int		`ArgName:"sessionExpire"`
	HttpHash		string   `ArgName:"httpHash"`	//wrr ip_hash least_conn
}
type ModifyLoadBalancerRulesProbeResponse struct {
	RequestId		int		`json:"requestId"`
}
//
func (client *Client) ModifyLoadBalancerRulesProbe(args *ModifyLoadBalancerRulesProbeArgs) (*ModifyLoadBalancerRulesProbeResponse, error) {
	response := &ModifyLoadBalancerRulesProbeResponse{}
	err := client.Invoke("ModifyLoadBalancerRulesProbe", args, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}