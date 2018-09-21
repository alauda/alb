package clb

type RegisterInstancesWithForwardLBSeventhListenerArgs struct {
	LoadBalancerId string `ArgName:"loadBalancerId"`
	ListenerId string `ArgName:"listenerId"`
	LocationIds	[]string `ArgName:"locationIds"`
	Backends    []RegisterInstancesOpts `ArgName:"backends"`
}

type RegisterInstancesWithForwardLBFourthListenerArgs struct {
	LoadBalancerId string `ArgName:"loadBalancerId"`
	ListenerId string `ArgName:"listenerId"`
	Backends    []RegisterInstancesOpts `ArgName:"backends"`
}

type RegisterInstancesOpts struct {
	InstanceId 	string `ArgName:"instanceId"`
	Port  		int	  `ArgName:"port"`
	Weight      int   `ArgName:"weight"`
}

type RegisterInstancesWithLoadBalancerResponse struct {
	RequestId int `json:"requestId"`
}

func (response RegisterInstancesWithLoadBalancerResponse) Id() int {
	return response.RequestId
}

func (client *Client) RegisterInstancesWithForwardLBSeventhListener(args *RegisterInstancesWithForwardLBSeventhListenerArgs) (
	*RegisterInstancesWithLoadBalancerResponse,
	error,
) {
	response := &RegisterInstancesWithLoadBalancerResponse{}
	err := client.Invoke("RegisterInstancesWithForwardLBSeventhListener", args, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (client *Client) RegisterInstancesWithForwardLBFourthListener(args *RegisterInstancesWithForwardLBFourthListenerArgs) (
	*RegisterInstancesWithLoadBalancerResponse,
	error,
) {
	response := &RegisterInstancesWithLoadBalancerResponse{}
	err := client.Invoke("RegisterInstancesWithForwardLBFourthListener", args, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

type DescribeLoadBalancerBackendsArgs struct {
	LoadBalancerId 		string `ArgName:"loadBalancerId"`
	LoadBalancerPort	int    `ArgName:"loadBalancerPort"`
}

type DescribeLoadBalancerBackendsResponse struct {
	Data	[]ListenerWithRules		`json:"data"`
}

type ListenerWithRules struct {
	ListenerId			string			`json:"listenerId"`
	Protocol			int				`json:"protocol"`
	ProtocolType		string			`json:"protocolType"`
	LoadBalancerPort	int				`json:"loadBalancerPort"`
	Rules 				[]SimpleRule 	`json:"rules"`
	Backends			[]LoadBalancerBackends	`json:"backends"`
}

type LoadBalancerBackends struct {
	UnInstanceId   string   `json:"unInstanceId"`
	Weight         int      `json:"weight"`
	InstanceName   string   `json:"instanceName"`
	Port 		   int		`json:"port"`
	LanIp          string   `json:"lanIp"`
	WanIpSet       []string `json:"wanIpSet"`
	InstanceStatus int      `json:"instanceStatus"`
}

func (client *Client) DescribeLoadBalancerBackends(args *DescribeLoadBalancerBackendsArgs) (
	*DescribeLoadBalancerBackendsResponse,
	error,
) {
	response := &DescribeLoadBalancerBackendsResponse{}
	err := client.Invoke("DescribeForwardLBBackends", args, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

type ModifyLoadBalancerBackendsArgs struct {
	LoadBalancerId string              `ArgName:"loadBalancerId,required"`
	Backends       []ModifyBackendOpts `ArgName:"backends,required"`
}

type ModifyBackendOpts struct {
	InstanceId string `ArgName:"instanceId,required"`
	Weight     int    `ArgName:"weight,required"`
}

type ModifyLoadBalancerBackendsResponse struct {
	RequestId int `json:"requestId"`
}

func (response ModifyLoadBalancerBackendsResponse) Id() int {
	return response.RequestId
}

func (client *Client) ModifyLoadBalancerBackends(args *ModifyLoadBalancerBackendsArgs) (
	*ModifyLoadBalancerBackendsResponse,
	error,
) {
	response := &ModifyLoadBalancerBackendsResponse{}
	err := client.Invoke("ModifyLoadBalancerBackends", args, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

type DeregisterInstancesFromForwardLBArgs struct {
	LoadBalancerId string `ArgName:"loadBalancerId"`
	ListenerId string `ArgName:"listenerId"`
	Backends    []RegisterInstancesOpts `ArgName:"backends"`
	LocationIds	[]string `ArgName:"locationIds"`
}

type DeregisterInstancesFromForwardLBFourthListenerArgs struct {
	LoadBalancerId string `ArgName:"loadBalancerId"`
	ListenerId string `ArgName:"listenerId"`
	Backends    []RegisterInstancesOpts `ArgName:"backends"`
}

type deRegisterBackend struct {
	InstanceId string `ArgName:"instanceId"`
}

type DeregisterInstancesFromLoadBalancerResponse struct {
	RequestId int `json:"requestId"`
}

func (response DeregisterInstancesFromLoadBalancerResponse) Id() int {
	return response.RequestId
}

func (client *Client) DeregisterInstancesFromForwardLB(args *DeregisterInstancesFromForwardLBArgs) (
	*DeregisterInstancesFromLoadBalancerResponse,
	error,
) {
	response := &DeregisterInstancesFromLoadBalancerResponse{}
	err := client.Invoke("DeregisterInstancesFromForwardLB", args, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}


func (client *Client) DeregisterInstancesFromForwardLBFourthListener(args *DeregisterInstancesFromForwardLBFourthListenerArgs) (
	*DeregisterInstancesFromLoadBalancerResponse,
	error,
) {

	response := &DeregisterInstancesFromLoadBalancerResponse{}
	err := client.Invoke("DeregisterInstancesFromForwardLBFourthListener", args, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}