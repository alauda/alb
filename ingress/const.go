package ingress

import (
	"alauda.io/alb2/config"
	"fmt"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Ingress is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	// MessageResourceSynced is the message used for an Event fired when a Ingress
	// is synced successfully
	MessageResourceSynced = "Ingress synced successfully"

	// ALBRewriteTargetAnnotation is the ingress annotation to define rewrite rule for alb
	ALBRewriteTargetAnnotation = "nginx.ingress.kubernetes.io/rewrite-target"
	// ALBEnableCORSAnnotation is the ingress annotation to enable cors for alb
	ALBEnableCORSAnnotation = "nginx.ingress.kubernetes.io/enable-cors"
	// ALBBackendProtocolAnnotation is the ingress annotation to define backend protocol
	ALBBackendProtocolAnnotation = "nginx.ingress.kubernetes.io/backend-protocol"

	// ALBTemporalRedirectAnnotation allows you to return a temporal redirect (Return Code 302) instead of sending data to the upstream.
	ALBTemporalRedirectAnnotation = "nginx.ingress.kubernetes.io/temporal-redirect"
	// ALBPermanentRedirectAnnotation allows to return a permanent redirect instead of sending data to the upstream.
	ALBPermanentRedirectAnnotation = "nginx.ingress.kubernetes.io/permanent-redirect"

	// ALBVHostAnnotation allows user to override the request Host
	ALBVHostAnnotation = "nginx.ingress.kubernetes.io/upstream-vhost"
)

var (
	AlwaysSSLStrategy  = "Always"
	NeverSSLStrategy   = "Never"
	RequestSSLStrategy = "Request"
	BothSSLStrategy    = "Both"
	DefaultSSLStrategy = RequestSSLStrategy

	ValidBackendProtocol = map[string]bool{
		"http":  true,
		"https": true,
	}
	// ALBSSLStrategyAnnotation allows you to use default ssl certificate for a http ingress
	ALBSSLStrategyAnnotation = fmt.Sprintf("alb.networking.%s/enable-https", config.Get("DOMAIN"))
	// ALBSSLAnnotation set https cert for host instead of using spec.tls
	ALBSSLAnnotation = fmt.Sprintf("alb.networking.%s/tls", config.Get("DOMAIN"))

	IngressHTTPPort  = config.GetInt("INGRESS_HTTP_PORT")
	IngressHTTPSPort = config.GetInt("INGRESS_HTTPS_PORT")

	DefaultPriority = 5
)
