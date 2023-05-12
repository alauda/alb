package ctl

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"

	. "alauda.io/alb2/gateway"
)

type Listener struct {
	gv1b1t.Listener
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
	ref    gv1b1t.ParentReference
	accept bool
	msg    string
	reason string
}

func (r *Route) invalidSectionName(ref gv1b1t.ParentReference, msg string) {
	key := RefsToString(ref)
	status := RouteStatus{
		ref:    ref,
		accept: false,
		msg:    msg,
	}
	r.status[key] = status
}

func (r *Route) accept(ref gv1b1t.ParentReference) {
	key := RefsToString(ref)
	status := RouteStatus{
		ref:    ref,
		accept: true,
		msg:    "",
	}
	r.status[key] = status
}

func (r *Route) unAllowRouteWithReason(ref gv1b1t.ParentReference, msg string, reason string) {
	key := RefsToString(ref)
	status := RouteStatus{
		ref:    ref,
		accept: false,
		msg:    msg,
		reason: reason,
	}
	r.status[key] = status
}
func (r *Route) unAllowRoute(ref gv1b1t.ParentReference, msg string) {
	r.unAllowRouteWithReason(ref, msg, "")
}

func (r *Route) invalidKind(ref gv1b1t.ParentReference, msg string) {
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
		string(gv1b1t.ListenerReasonProtocolConflict),
		msg,
	}
}

func (l ListenerStatus) toConditions(gateway *gv1b1t.Gateway) []metav1.Condition {
	if l.valid {
		return []metav1.Condition{
			{
				Type:               string(gv1b1t.ListenerConditionReady),
				Status:             metav1.ConditionTrue,
				Reason:             string(gv1b1t.ListenerReasonReady),
				LastTransitionTime: metav1.Now(),
				ObservedGeneration: gateway.Generation,
			},
		}
	}
	conditions := make([]metav1.Condition, 0)
	conditions = append(conditions, metav1.Condition{
		Type:               string(gv1b1t.ListenerConditionReady),
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: gateway.Generation,
		Status:             metav1.ConditionFalse,
		Reason:             string(gv1b1t.ListenerReasonInvalid),
	})

	if l.conflicted != nil {
		conditions = append(conditions, metav1.Condition{
			Type:               string(gv1b1t.ListenerConditionConflicted),
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: gateway.Generation,
			Status:             metav1.ConditionTrue,
			Reason:             l.conflicted.reason,
			Message:            l.conflicted.msg,
		})
	}
	if l.detached != nil {
		conditions = append(conditions, metav1.Condition{
			Type:               string(gv1b1t.ListenerConditionDetached),
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: gateway.Generation,
			Status:             metav1.ConditionTrue,
			Reason:             l.conflicted.reason,
			Message:            l.conflicted.msg,
		})
	}
	if l.resolvedRefs != nil {
		conditions = append(conditions, metav1.Condition{
			Type:               string(gv1b1t.ListenerConditionDetached),
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
func RefsToString(ref gv1b1t.ParentReference) string {
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
