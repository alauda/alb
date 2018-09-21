package common

import (
	"bitbucket.org/mathildetech/qcloud-sdk-go/util"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

const (
	RequestMethodGET  = "GET"
	RequestMethodPOST = "POST"
)

// A Client represents a client of QCloud services
type Client struct {
	httpClient *http.Client
	credential Credential
	opts       Opts
}

type Opts struct {
	Method       string
	Region       string
	Endpoint     string
	Debug        bool
	Version      string
	LionUser     string //a api proxy for some customers do not want to expose secret
	LionPassword string // id and secret key,use LionUser and LionPwd to certify for third components
}

type Credential struct {
	SecretId  string
	SecretKey string
}

func NewClient(credential Credential, opts Opts) (*Client, error) {
	if opts.Method == "" {
		opts.Method = RequestMethodGET
	}
	return &Client{
		&http.Client{},
		credential,
		opts,
	}, nil
}

func (client *Client) Invoke(action string, args interface{}, response interface{}) error {
	switch client.opts.Method {
	case RequestMethodGET:
		return client.InvokeGet(action, args, response)
	default:
		return client.InvokePost(action, args, response)
	}
}

// Invoke sends the raw HTTP request for Q_Cloud services
func (client *Client) InvokeGet(action string, args interface{}, response interface{}) error {
	reqValues := url.Values{}
	reqValues.Set("Action", action)
	reqValues.Set("Region", client.opts.Region)
	reqValues.Set("Timestamp", fmt.Sprint(uint(time.Now().Unix())))
	reqValues.Set("Nonce", fmt.Sprint(uint(rand.Int())))
	reqValues.Set("SecretId", client.credential.SecretId)
	err := util.EncodeStruct(args, &reqValues)
	if client.opts.Debug {
		glog.Infoln(reqValues)
	}
	if err != nil {
		return makeClientError(err)
	}
	// Sign request
	signature := util.CreateSignatureForRequest(client.opts.Method, &reqValues, client.opts.Endpoint, client.credential.SecretKey)

	// Generate the request URL
	requestURL := client.opts.Endpoint + "?" + reqValues.Encode() + "&Signature=" + url.QueryEscape(signature)

	httpReq, err := http.NewRequest(client.opts.Method, requestURL, nil)
	if err != nil {
		return GetClientError(err)
	}
	if client.opts.LionUser != "" && client.opts.LionPassword != "" {
		httpReq.Header.Add("Accept", "application/json")
		httpReq.SetBasicAuth(client.opts.LionUser, client.opts.LionPassword)
	}

	t0 := time.Now()
	httpResp, err := client.httpClient.Do(httpReq)
	t1 := time.Now()
	if err != nil {
		return GetClientError(err)
	}
	statusCode := httpResp.StatusCode

	if client.opts.Debug {
		glog.Infoln("Invoke %s %s %d (%v)", RequestMethodGET, requestURL, statusCode, t1.Sub(t0))
	}

	defer httpResp.Body.Close()
	body, err := ioutil.ReadAll(httpResp.Body)

	if err != nil {
		return GetClientError(err)
	}

	if client.opts.Debug {
		var prettyJSON bytes.Buffer
		err = json.Indent(&prettyJSON, body, "", "    ")
		glog.Infoln(string(prettyJSON.Bytes()))
	}

	if statusCode >= 400 && statusCode <= 599 {
		errorResponse := HttpStatusCodeResponse{}
		err = json.Unmarshal(body, &errorResponse)
		ecsError := &Error{
			HttpStatusCodeResponse: errorResponse,
			StatusCode:             statusCode,
		}
		return ecsError
	}
	legacyErrorResponse := LegacyAPIError{}

	if err = json.Unmarshal(body, &legacyErrorResponse); err != nil {
		return makeClientError(err)
	}

	versionErrorResponse := VersionAPIError{}

	if err = json.Unmarshal(body, &versionErrorResponse); err != nil {
		return makeClientError(err)
	}

	if legacyErrorResponse.Code != NoErr || (legacyErrorResponse.CodeDesc != "" && legacyErrorResponse.CodeDesc != NoErrCodeDesc) {
		return legacyErrorResponse
	}

	if versionErrorResponse.Response.Error.Code != "" {
		return versionErrorResponse
	}
	err = json.Unmarshal(body, response)
	if err != nil {
		return GetClientError(err)
	}
	return nil
}

//todo
func (client *Client) InvokePost(action string, args interface{}, response interface{}) error {
	return nil
}

func GetClientErrorFromString(str string) error {
	return &Error{
		HttpStatusCodeResponse: HttpStatusCodeResponse{
			Code:    "QCloudGoClientFailure",
			Message: str,
		},
		StatusCode: -1,
	}
}

func GetClientError(err error) error {
	return GetClientErrorFromString(err.Error())
}
