package ctl

type CommonRouteConditionReason string

const (
	CommonRouteReasonInvalidSectionName CommonRouteConditionReason = "InvalidSectionName"
	CommonRouteReasonUnAllowRoute       CommonRouteConditionReason = "UnAllowRoute"
	CommonRouteReasonInvalidKind        CommonRouteConditionReason = "InvalidKind"
)
