package types

const (
	SubsystemHTTP   = "http"
	SubsystemStream = "stream"

	PolicySIPHash = "sip-hash"
	PolicyCookie  = "cookie"

	CaCert = "ca.crt"
)

var (
	LastConfig  = ""
	LastFailure = false
)

const (
	ModeTCP  = "tcp"
	ModeHTTP = "http"
	ModeUDP  = "udp"
	ModegRPC = "grpc"
)

const (
	RuleTypeIngress = "ingress"
	RuleTypeGateway = "gateway"
)
