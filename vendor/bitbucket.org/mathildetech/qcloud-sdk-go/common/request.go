package common

import (
	"math/rand"
	"time"
)

// Constants for QCloud API requests
type Request struct {
	Region      string
	AccessKeyId string `ArgName:"SecretId"`
	Timestamp   int
	Nonce       int
	Action      string
}

type Response struct {
	RequestId int
}

func (request *Request) init(action, accessKeyId, region string) {
	request.Region = region
	request.Timestamp = int(time.Now().Unix())
	request.Nonce = rand.New(rand.NewSource(time.Now().Unix())).Intn(100000)
	request.Action = action
	request.AccessKeyId = accessKeyId
}

type Pagination struct {
	PageNumber int
	PageSize   int
}

func (p *Pagination) SetPageSize(size int) {
	p.PageSize = size
}

func (p *Pagination) Validate() {
	if p.PageNumber < 0 {
		p.PageNumber = 1
	}
	if p.PageSize < 0 {
		p.PageSize = 10
	} else if p.PageSize > 50 {
		p.PageSize = 50
	}
}

// A PaginationResponse represents a response with pagination information
type PaginationResult struct {
	TotalCount int
	PageNumber int
	PageSize   int
}

// NextPage gets the next page of the result set
func (r *PaginationResult) NextPage() *Pagination {
	if r.PageNumber*r.PageSize >= r.TotalCount {
		return nil
	}
	return &Pagination{PageNumber: r.PageNumber + 1, PageSize: r.PageSize}
}
