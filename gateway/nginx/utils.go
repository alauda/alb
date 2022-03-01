package nginx

import (
	"fmt"

	"alauda.io/alb2/utils"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"

	. "alauda.io/alb2/controller/types"
)

// translate gateway matchType.to alb op
// Exact => EQ
// PathPrefix => STARTS_WITH
// RegularExpression => REGEX
// nil => EQ
// otherwise return err
func toOP(matchType *string) (string, error) {
	if matchType == nil {
		return utils.OP_EQ, nil
	}
	switch *matchType {
	case "Exact":
		return utils.OP_EQ, nil
	case "PathPrefix":
		return utils.OP_STARTS_WITH, nil
	case "RegularExpression":
		return utils.OP_REGEX, nil
	default:
		return "", fmt.Errorf("unsupported match type %v", matchType)
	}
}

func backendRefsToService(refs []gatewayType.BackendRef) ([]*BackendService, error) {
	svcs := []*BackendService{}
	for _, ref := range refs {
		kind := ref.Kind
		if kind != nil && *kind != "Service" {
			return nil, fmt.Errorf("gateway: backend ref kind is not service but %v", kind)
		}
		if ref.Namespace == nil || ref.Port == nil || ref.Weight == nil {
			return nil, fmt.Errorf("invalid ref %v", ref)
		}
		svcs = append(svcs, &BackendService{
			ServiceNs:   string(*ref.Namespace),
			ServiceName: string(ref.Name),
			ServicePort: int(*ref.Port),
			Weight:      int(*ref.Weight),
		})
	}
	return svcs, nil
}
