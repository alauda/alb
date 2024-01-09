package ctl

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "alauda.io/alb2/gateway"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type Listener struct {
	gv1.Listener
	gateway    client.ObjectKey
	createTime time.Time
	version    int64
	status     ListenerStatus
}

type ListenerStatus struct {
	valid          bool
	allKindInvalid bool
	attachedRoutes int32
	conflicted     *struct {
		reason string // hostname/protocol/route
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
	ref    gv1.ParentReference
	accept bool
	msg    string
	reason string
}

func (r *Route) invalidSectionName(ref gv1.ParentReference, msg string) {
	key := RefsToString(ref)
	status := RouteStatus{
		ref:    ref,
		accept: false,
		msg:    msg,
	}
	r.status[key] = status
}

func (r *Route) accept(ref gv1.ParentReference) {
	key := RefsToString(ref)
	status := RouteStatus{
		ref:    ref,
		accept: true,
		msg:    "",
	}
	r.status[key] = status
}

func (r *Route) unAllowRouteWithReason(ref gv1.ParentReference, msg string, reason string) {
	key := RefsToString(ref)
	status := RouteStatus{
		ref:    ref,
		accept: false,
		msg:    msg,
		reason: reason,
	}
	r.status[key] = status
}

func (r *Route) unAllowRoute(ref gv1.ParentReference, msg string) {
	r.unAllowRouteWithReason(ref, msg, "")
}

func (r *Route) invalidKind(ref gv1.ParentReference, msg string) {
	key := RefsToString(ref)
	status := RouteStatus{
		ref:    ref,
		accept: false,
		msg:    msg,
	}
	r.status[key] = status
}

func (l *ListenerStatus) conflictProtocol(msg string) {
	l.valid = false
	l.conflicted = &struct {
		reason string
		msg    string
	}{
		string(gv1.ListenerReasonProtocolConflict),
		msg,
	}
}

func (l *ListenerStatus) invalidKind(allinvalid bool, invalidkinds []string) {
	l.valid = false
	l.allKindInvalid = allinvalid
	l.resolvedRefs = &struct {
		reason string
		msg    string
	}{
		reason: string(gv1.ListenerReasonInvalidRouteKinds),
		msg:    fmt.Sprintf("invalid kinds %v", invalidkinds),
	}
}

func (l ListenerStatus) toConditions(gateway *gv1.Gateway) []metav1.Condition {
	if l.valid {
		return []metav1.Condition{
			{
				Type:               string(gv1.ListenerConditionReady),
				Status:             metav1.ConditionTrue,
				Reason:             string(gv1.ListenerReasonReady),
				LastTransitionTime: metav1.Now(),
				ObservedGeneration: gateway.Generation,
			},
		}
	}
	conditions := make([]metav1.Condition, 0)
	conditions = append(conditions, metav1.Condition{
		Type:               string(gv1.ListenerConditionReady),
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: gateway.Generation,
		Status:             metav1.ConditionFalse,
		Reason:             string(gv1.ListenerReasonInvalid),
	})

	if l.conflicted != nil {
		conditions = append(conditions, metav1.Condition{
			Type:               string(gv1.ListenerConditionConflicted),
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: gateway.Generation,
			Status:             metav1.ConditionTrue,
			Reason:             l.conflicted.reason,
			Message:            l.conflicted.msg,
		})
	}

	if l.resolvedRefs != nil {
		conditions = append(conditions, metav1.Condition{
			Type:               string(gv1.ListenerConditionResolvedRefs),
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: gateway.Generation,
			Status:             metav1.ConditionFalse,
			Reason:             l.resolvedRefs.reason,
			Message:            l.resolvedRefs.msg,
		})
	}
	return conditions
}

// TODO move to common
func RefsToString(ref gv1.ParentReference) string {
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
