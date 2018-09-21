package cvm

type DescribeInstanceArgs struct {
	LanIp string `ArgName:"lanIps.0"`
}

type InstanceType struct {
	InstanceName string
	UnInstanceId string
	LanIp        string
	WanIpSet     []string
	Region       string
	Status       int
}

type DescribeInstanceResponse struct {
	InstanceSet []*InstanceType
}

func (client *Client) DescribeInstances(args *DescribeInstanceArgs) ([]*InstanceType, error) {
	response := DescribeInstanceResponse{}
	err := client.Invoke("DescribeInstances", args, &response)
	if err == nil {
		return response.InstanceSet, nil
	}
	return nil, err
}
