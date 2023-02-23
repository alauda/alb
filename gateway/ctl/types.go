package ctl

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"

	. "alauda.io/alb2/gateway"
)

type Listener struct {
	gatewayType.Listener
	gateway    client.ObjectKey
	createTime time.Time
	version    int64
	status     ListenerStatus
}

type ListenerStatus struct {
	valid          bool
	attachedRoutes int32
	allowedRoutes  int
	conflicted     *struct {
		reason string // hostname/protocol/route
		msg    string
	}
	detached *struct {
		reason string // protunavaiable
		msg    string
	}
	resolvedRefs *struct {
		reason string // InvalidCertificateRef/InvalidRouteKinds/RefNotPermitted
		msg    string
	}
}

type Route struct {
	route  CommonRoute
	status map[string]RouteStatus
}

type RouteStatus struct {
	ref    gatewayType.ParentRef
	accept bool
	msg    string
}

func (r *Route) invalidSectionName(ref gatewayType.ParentRef, msg string) {
	key := RefsToString(ref)
	status := RouteStatus{
		ref:    ref,
		accept: false,
		msg:    msg,
	}
	r.status[key] = status
}

func (r *Route) accept(ref gatewayType.ParentRef) {
	key := RefsToString(ref)
	status := RouteStatus{
		ref:    ref,
		accept: true,
		msg:    "",
	}
	r.status[key] = status
}

func (r *Route) unAllowRoute(ref gatewayType.ParentRef, msg string) {
	key := RefsToString(ref)
	status := RouteStatus{
		ref:    ref,
		accept: false,
		msg:    msg,
	}
	r.status[key] = status
}

func (r *Route) invalidKind(ref gatewayType.ParentRef, msg string) {
	key := RefsToString(ref)
	status := RouteStatus{
		ref:    ref,
		accept: false,
		msg:    msg,
	}
	r.status[key] = status
}

func (l *ListenerStatus) conflictProtocol(msg string) {
	l.conflicted = &struct {
		reason string
		msg    string
	}{
		string(gatewayType.ListenerReasonProtocolConflict),
		msg,
	}
}

func (l ListenerStatus) toConditions(gateway *gatewayType.Gateway) []metav1.Condition {
	if l.valid {
		return []metav1.Condition{
			{
				Type:               string(gatewayType.ListenerConditionReady),
				Status:             metav1.ConditionTrue,
				Reason:             string(gatewayType.ListenerReasonReady),
				LastTransitionTime: metav1.Now(),
				ObservedGeneration: gateway.Generation,
			},
		}
	}
	conditions := make([]metav1.Condition, 0)
	conditions = append(conditions, metav1.Condition{
		Type:               string(gatewayType.ListenerConditionReady),
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: gateway.Generation,
		Status:             metav1.ConditionFalse,
		Reason:             string(gatewayType.ListenerReasonInvalid),
	})

	if l.conflicted != nil {
		conditions = append(conditions, metav1.Condition{
			Type:               string(gatewayType.ListenerConditionConflicted),
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: gateway.Generation,
			Status:             metav1.ConditionTrue,
			Reason:             l.conflicted.reason,
			Message:            l.conflicted.msg,
		})
	}
	if l.detached != nil {
		conditions = append(conditions, metav1.Condition{
			Type:               string(gatewayType.ListenerConditionDetached),
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: gateway.Generation,
			Status:             metav1.ConditionTrue,
			Reason:             l.conflicted.reason,
			Message:            l.conflicted.msg,
		})
	}
	if l.resolvedRefs != nil {
		conditions = append(conditions, metav1.Condition{
			Type:               string(gatewayType.ListenerConditionDetached),
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: gateway.Generation,
			Status:             metav1.ConditionFalse,
			Reason:             l.conflicted.reason,
			Message:            l.conflicted.msg,
		})
	}

	return conditions
}

// TODO move to common
func RefsToString(ref gatewayType.ParentRef) string {
	kind := ""
	if ref.Kind != nil {
		kind = string(*ref.Kind)
	}
	ns := ""
	if ref.Namespace != nil {
		ns = string(*ref.Namespace)
	}
	sectionName := ""
	if ref.SectionName != nil {
		sectionName = string(*ref.SectionName)
	}
	return fmt.Sprintf("%s/%s/%s/%s", kind, ns, ref.Name, sectionName)
}
