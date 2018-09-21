package clb

const (
	LoadBalanceListenerProtocolHTTP  = 1
	LoadBalanceListenerProtocolTCP   = 2
	LoadBalanceListenerProtocolUDP   = 3
	LoadBalanceListenerProtocolHTTPS = 4
)

type CreateFourLayerLoadBalancerListenersArgs struct {
	LoadBalancerId string               `ArgName:"loadBalancerId"`
	Listeners      []CreateFourLayerListenerOpts `ArgName:"listeners"`
}

type CreateFourLayerListenerOpts struct {
	LoadBalancerPort int   		`ArgName:"loadBalancerPort"`
	Protocol         int     	`ArgName:"protocol"`
	ListenerName     string 	`ArgName:"listenerName"`
	SessionExpire    int    	`ArgName:"sessionExpire"`
	HealthSwitch     int    	`ArgName:"healthSwitch"`
	TimeOut          int    	`ArgName:"timeOut"`
	IntervalTime     int    	`ArgName:"intervalTime"`
	HealthNum        int    	`ArgName:"healthNum"`
	UnhealthNum      int    	`ArgName:"unhealthNum"`
	Scheduler		 string	 	`ArgName:"scheduler"`
}

type CreateLoadBalancerListenersResponse struct {
	RequestId   int      `json:"requestId"`
	ListenerIds []string `json:"listenerIds"`
}

type CreateSevenLayerLoadBalancerListenersArgs struct {
	LoadBalancerId string               			`ArgName:"loadBalancerId"`
	Listeners      []CreateSevenLayerListenerOpts 	`ArgName:"listeners"`
}

type CreateSevenLayerListenerOpts struct {
	LoadBalancerPort int   	 	`ArgName:"loadBalancerPort"`
	Protocol         int     	`ArgName:"protocol"`
	ListenerName     string 	`ArgName:"listenerName"`
	SSLMode    		 string 	`ArgName:"SSLMode"`
	CertId     		 string    	`ArgName:"certId"`
	CertCaId         string    	`ArgName:"certCaId"`
	CertCaName     	 string    	`ArgName:"certCaName"`
	CertContent      string    	`ArgName:"certContent"`
	CertKey      	 string  	`ArgName:"certKey"`
	CertName		 string	 	`ArgName:"certName"`
}

func (response CreateLoadBalancerListenersResponse) Id() int {
	return response.RequestId
}

func (client *Client) CreateFourLayerLoadBalancerListeners(args *CreateFourLayerLoadBalancerListenersArgs) (
	*CreateLoadBalancerListenersResponse,
	error,
) {
	response := &CreateLoadBalancerListenersResponse{}
	err := client.Invoke("CreateForwardLBFourthLayerListeners", args, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (client *Client) CreateSevenLayerLoadBalancerListeners(args *CreateSevenLayerLoadBalancerListenersArgs) (
	*CreateLoadBalancerListenersResponse,
	error,
) {
	response := &CreateLoadBalancerListenersResponse{}
	err := client.Invoke("CreateForwardLBSeventhLayerListeners", args, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

type DescribeForwardLBListenersArgs struct {
	LoadBalancerId   string    `ArgName:"loadBalancerId"`
	ListenerIds      []string `ArgName:"listenerIds"`
//	Protocol         int      `ArgName:"protocol"`
//	LoadBalancerPort int    `ArgName:"loadBalancerPort"`
}

type DescribeForwardLBListenersResponse struct {
	TotalCount  int        `json:"totalCount"`
	ListenerSet []Listener `json:"listenerSet"`
}

type Listener struct {
	ListenerId     	 string 		`json:"listenerId"`
	LoadBalancerPort int  		`json:"loadBalancerPort"`
	Protocol         int  	  	`json:"protocol"`
	ProtocolType	 string  	`json:"protocolType"`
	SSLMode          string 	`json:"SSLMode"`
	CertId           string 	`json:"certId"`
	CertCaId         string 	`json:"certCaId"`
	Status           int    	`json:"status"`
	Rules			 []Rule		`json:"rules"`
}

func (client *Client) DescribeForwardLoadBalancerListeners(args *DescribeForwardLBListenersArgs) (
	*DescribeForwardLBListenersResponse,
	error,
) {
	response := &DescribeForwardLBListenersResponse{}
	err := client.Invoke("DescribeForwardLBListeners", args, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

type DeleteLoadBalancerListenersArgs struct {
	LoadBalancerId string   `ArgName:"loadBalancerId"`
	ListenerId    string `ArgName:"listenerId"`
}

type DeleteLoadBalancerListenersResponse struct {
	RequestId int `json:"requestId"`
}

func (response DeleteLoadBalancerListenersResponse) Id() int {
	return response.RequestId
}

func (client *Client) DeleteLoadBalancerListeners(LoadBalancerId string, ListenerId string) (*DeleteLoadBalancerListenersResponse, error) {
	response := &DeleteLoadBalancerListenersResponse{}
	err := client.Invoke("DeleteForwardLBListener", &DeleteLoadBalancerListenersArgs{
		LoadBalancerId: LoadBalancerId,
		ListenerId:    ListenerId,
	}, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

type ModifyLoadBalancerListenerArgs struct {
	LoadBalancerId string  `ArgName:"loadBalancerId"`
	ListenerId     string  `ArgName:"listenerId"`
	ListenerName   *string `ArgName:"listenerName"`
	SessionExpire  *int    `ArgName:"sessionExpire"`
	HealthSwitch   *int    `ArgName:"healthSwitch"`
	TimeOut        *int    `ArgName:"timeOut"`
	IntervalTime   *int    `ArgName:"intervalTime"`
	HealthNum      *int    `ArgName:"healthNum"`
	UnhealthNum    *int    `ArgName:"unhealthNum"`
	HttpHash       *int    `ArgName:"httpHash"`
	HttpCode       *int    `ArgName:"httpCode"`
	HttpCheckPath  *string `ArgName:"httpCheckPath"`
	SSLMode        *string `ArgName:"SSLMode"`
	CertId         *string `ArgName:"certId"`
	CertCaId       *string `ArgName:"certCaId"`
	CertCaContent  *string `ArgName:"certCaContent"`
	CertCaName     *string `ArgName:"certCaName"`
	CertContent    *string `ArgName:"certContent"`
	CertKey        *string `ArgName:"certKey"`
	CertName       *string `ArgName:"certName"`
}

type ModifyLoadBalancerListenerResponse struct {
	RequestId int `json:"requestId"`
}

func (response ModifyLoadBalancerListenerResponse) Id() int {
	return response.RequestId
}

func (client *Client) ModifyLoadBalancerListener(args *ModifyLoadBalancerListenerArgs) (
	*ModifyLoadBalancerListenerResponse,
	error,
) {
	response := &ModifyLoadBalancerListenerResponse{}
	err := client.Invoke("ModifyLoadBalancerListener", args, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}
