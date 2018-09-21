package common

import "fmt"

const (
	NoErr         = 0
	NoErrCodeDesc = "Success"

	ErrQCloudGoClient = 9999
)

type LegacyAPIError struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	CodeDesc string `json:"codeDesc"`
}

func (lae LegacyAPIError) Error() string {
	return lae.Message
}

type VersionAPIError struct {
	Response struct {
		Error apiErrorResponse `json:"Error"`
	} `json:"Response"`
}

type apiErrorResponse struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

func (vae VersionAPIError) Error() string {
	return vae.Response.Error.Message
}

type HttpStatusCodeResponse struct {
	Response
	HostId  string
	Code    string
	Message string
}

// An Error represents a custom error for QCloud API failure response
type Error struct {
	HttpStatusCodeResponse
	StatusCode int //Status Code of HTTP Response
}

func (e *Error) Error() string {
	return fmt.Sprintf("QCloud API Error: RequestId: %s Status Code: %d Code: %s Message: %s", e.RequestId, e.StatusCode, e.Code, e.Message)
}

type ClientError struct {
	Message string
}

func (ce ClientError) Error() string {
	return ce.Message
}

func makeClientError(err error) ClientError {
	return ClientError{
		Message: err.Error(),
	}
}
