package cvm

import "bitbucket.org/mathildetech/qcloud-sdk-go/common"

const (
	// CLBDefaultEndpoint is the default API endpoint of CVM services
	CVMDefaultEndpoint = "https://cvm.api.qcloud.com/v2/index.php"
	CVMAPIVersion      = "2016-10-28"
)

// A Client represents a client of CLB
type Client struct {
	*common.Client
}

// NewClient creates a new instance of CLB client
func NewClient(credential common.Credential, opts common.Opts) (*Client, error){
	if opts.Endpoint == "" {
		opts.Endpoint = CVMDefaultEndpoint
	}
	if opts.Version == "" {
		opts.Version = CVMAPIVersion
	}
	opts.Debug = true
	client, err := common.NewClient(credential, opts)
	if err != nil {
		return &Client{}, err
	}
	return &Client{client}, nil
}

