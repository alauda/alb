package clb

import (
	"bitbucket.org/mathildetech/qcloud-sdk-go/common"
)

const (
	InternalLoadBalancer = 3
	ExternalLoadBalancer = 2
)

type CreateLoadBalancerArgs struct {
	LoadBalancerName string `ArgName:"loadBalancerName"`
	LoadBalancerType int    `ArgName:"loadBalancerType"`
	Forward			 int	`ArgName:"forward"`		// 0: traditional  1: application, default 0
	VpcId            string    `ArgName:"vpcId"`
	SubnetId         string    `ArgName:"subnetId"`
	ProjectId        int    `ArgName:"projectId"`
}

type CreateLoadBalancerResponse struct {
	UnLoadBalancerIds map[string]interface{}
	DealIds           []string
	RequestId         int         `json:"requestId"`
}

type DescribeLoadBalancersArgs struct {
	LoadBalancerIds    []string `ArgName:"loadBalancerIds"`
	AddressType        int		`ArgName:"addressType"`
	NetworkType        string	`ArgName:"networkType"`
	VpcId              string	`ArgName:"vpcId"`
	VSwitchId          string	`ArgName:"vSwitchId"`
	Address            string	`ArgName:"address"`
	InternetChargeType string	`ArgName:"internetChargeType"`
	ServerId           string	`ArgName:"serverId"`
}

type DescribeLoadBalancerResponse struct {
	LoadBalancerId   string
	UnLoadBalancerId string
	LoadBalancerName string
	LoadBalancerType int
	Domain           string
	Forward			 int
	LoadBalancerVips []string
	Status           int
	CreateTime       string
	StatusTime       string
	ProjectId        int
	VpcId            int
	SubnetId         int
}

type DescribeLoadBalancerSetResponse struct {
	LoadBalancerSet []*DescribeLoadBalancerResponse
}

type DeleteLoadBalancerArgs struct {
	LoadBalancerId string `ArgName:"loadBalancerIds.1"`
}

type DeleteLoadBalancerResponse struct {
	common.Response
}

type DescribeHealthStatusArgs struct {
	LoadBalancerId string `ArgName:"loadBalancerIds.1"`
}

type HealthStatusLb struct {
	LoadBalancerName	string
	LoadBalancerId		string
	UnLoadBalancerId	string
	Listener			[]HealthStatusListener
}

type HealthStatusListener struct {
	ListenerId			string
	Protocol			int
	LoadBalancerPort	int
	ListenerName		string
	Rules				[]HealthStatusRule
}

type HealthStatusRule struct {
	LocationId		string
	Domain			string
	Url				string
	Backends		[]HealthStatusBackend
}

type HealthStatusBackend struct {
	Ip				string
	Port			int
	HealthStatus	int		// Health status: 1 -- healthy, 0 -- unhealthy, -1 -- unknown
}

type DescribeHealthStatusResponse struct {
	Data []HealthStatusLb
}

type DescribeLoadBalancersTaskResultArgs struct {
	RequestId int `ArgName:"requestId"`
}

func (response DescribeLoadBalancersTaskResultArgs) Id() int {
	return response.RequestId
}


type DescribeLoadBalancersTaskResultResponse struct {
	Data struct {
		Status int `json:"status"`
	} `json:"data"`
}

func (client *Client) CreateLoadBalancer(args *CreateLoadBalancerArgs) (*CreateLoadBalancerResponse, error) {
	createResponse := &CreateLoadBalancerResponse{}
	err := client.Invoke("CreateLoadBalancer", args, createResponse)
	if err != nil {
		return &CreateLoadBalancerResponse{}, err
	}
	// loadBalancerIds := createResponse.UnLoadBalancerIds[createResponse.DealIds[0]].([]interface{})
	// loadBalancerId = loadBalancerIds[0].(string)
	return createResponse, err
}

func (client *Client) DescribeLoadBalancer(loadBalancerId string) (res *DescribeLoadBalancerResponse, err error) {
	loadBalancerIds := []string{loadBalancerId}
	args := DescribeLoadBalancersArgs{
		LoadBalancerIds: loadBalancerIds,
	}
	response := &DescribeLoadBalancerSetResponse{}
	ok := client.Invoke("DescribeLoadBalancers", args, &response)
	if ok != nil{
		return &DescribeLoadBalancerResponse{}, err
	}
	for _, item := range response.LoadBalancerSet {
		if item.UnLoadBalancerId == loadBalancerId {
			res = item
			break
		}
	}
	return res, err
}

func (client *Client) DeleteLoadBalancer(loadBalancerId string) error {
	args := &DeleteLoadBalancerArgs{
		LoadBalancerId: loadBalancerId,
	}
	response := &DeleteLoadBalancerResponse{}
	err := client.Invoke("DeleteLoadBalancers", args, response)
	return err
}

func (client *Client) DescribeHealthStatus(args *DescribeHealthStatusArgs) (response *DescribeHealthStatusResponse, err error) {
	response = &DescribeHealthStatusResponse{}
	err = client.Invoke("DescribeForwardLBHealthStatus", args, response)
	if err != nil {
		return nil, err
	}
	return response, err
}

func (client *Client) DescribeLoadBalancersTaskResult(taskId int) (*DescribeLoadBalancersTaskResultResponse, error) {
	args := &DescribeLoadBalancersTaskResultArgs{
		RequestId: taskId,
	}
	response := &DescribeLoadBalancersTaskResultResponse{}
	err := client.Invoke("DescribeLoadBalancersTaskResult", args, response)
	if err != nil {
		return &DescribeLoadBalancersTaskResultResponse{}, err
	}
	return response, nil
}
