package clb

import (
	"bitbucket.org/mathildetech/qcloud-sdk-go/common"
)

const (
	// CLBDefaultEndpoint is the default API endpoint of CLB services
	CLBDefaultEndpoint = "https://lb.api.qcloud.com/v2/index.php"
	CLBAPIVersion      = "2017-12-15"
)

// A Client represents a client of CLB
type Client struct {
	*common.Client
}

// NewClient creates a new instance of CLB client
func NewClient(credential common.Credential, opts common.Opts) (*Client, error){
	if opts.Endpoint == "" {
		opts.Endpoint = CLBDefaultEndpoint
	}
	if opts.Version == "" {
		opts.Version = CLBAPIVersion
	}
//	opts.Debug = true
	client, err := common.NewClient(credential, opts)
	if err != nil {
		return &Client{}, err
	}
	return &Client{client}, nil
}
