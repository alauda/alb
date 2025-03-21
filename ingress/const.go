package ingress

const (
	// SuccessSynced is used as part of the Event 'reason' when an Ingress is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	// MessageResourceSynced is the message used for an Event fired when an Ingress
	// is synced successfully
	MessageResourceSynced = "Ingress synced successfully"

	// ALBRewriteTargetAnnotation is the ingress annotation to define rewrite rule for alb
	ALBRewriteTargetAnnotation = "nginx.ingress.kubernetes.io/rewrite-target"
	// ALBEnableCORSAnnotation is the ingress annotation to enable cors for alb
	ALBEnableCORSAnnotation = "nginx.ingress.kubernetes.io/enable-cors"
	// ALBCORSAllowHeadersAnnotation is the ingress annotation to configure cors allow headers
	ALBCORSAllowHeadersAnnotation = "nginx.ingress.kubernetes.io/cors-allow-headers"
	// ALBCORSAllowOriginAnnotation is the ingress annotation to configure cors allow origin
	ALBCORSAllowOriginAnnotation = "nginx.ingress.kubernetes.io/cors-allow-origin"
	// ALBBackendProtocolAnnotation is the ingress annotation to define backend protocol
	ALBBackendProtocolAnnotation  = "nginx.ingress.kubernetes.io/backend-protocol"
	FMT_ALBRulePriorityAnnotation = "alb.%s/ingress-rule-priority-%d-%d"

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
	DefaultPriority = 5
)
