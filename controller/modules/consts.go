package modules

import alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"

const (
	// ProtoHTTP is the protocol of http frontend
	ProtoHTTP  = alb2v1.FtProtocolHTTP
	ProtoHTTPS = alb2v1.FtProtocolHTTPS
)

const (
	TypeBind      = "bind"
	TypeIngress   = "ingress"
	TypeHttpRoute = "httpRoute"
	TypeTCPRoute  = "tcpRoute"
	TypeUDPRoute  = "udpRoute"
)

const (
	ProjectALL = "ALL_ALL"
)

type AlbPhase string

const (
	PhaseStarting    AlbPhase = "starting"
	PhaseRunning     AlbPhase = "running"
	PhaseTerminating AlbPhase = "terminating"
)
