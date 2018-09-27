package modules

type AlaudaLoadBalancer struct {
	Alb2Spec
	Name      string
	Namespace string
	Frontends []*Frontend
}

type Frontend struct {
	Name string
	FrontendSpec
	Rules []*Rule
}

type Rule struct {
	RuleSpec
	Name string
}
