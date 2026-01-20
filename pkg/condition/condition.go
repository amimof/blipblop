package condition

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

const (
	RuntimeReady Type = "RuntimeReady"
	ImageReady   Type = "ImageReady"
	VolumeReady  Type = "VolumeReady"
	NetworkReady Type = "NetworkReady"
	TaskReady    Type = "TaskReady"

	ReasonPulling    Reason = "Pulling"
	ReasonPulled     Reason = "Pulled"
	ReasonPullFailed Reason = "PullFailed"

	ReasonAttaching    Reason = "Attaching"
	ReasonAttached     Reason = "Attached"
	ReasonAttachFailed Reason = "AttachFailed"

	ReasonDetaching    Reason = "Detaching"
	ReasonDetached     Reason = "Detached"
	ReasonDetachFailed Reason = "DetachFailed"

	ReasonCreating     Reason = "Creating"
	ReasonCreated      Reason = "Created"
	ReasonCreateFailed Reason = "CreateFailed"

	ReasonStarting    Reason = "Starting"
	ReasonStarted     Reason = "Started"
	ReasonStartFailed Reason = "StartFailed"

	ReasonStopping   Reason = "Stopping"
	ReasonStopped    Reason = "Stopped"
	ReasonStopFailed Reason = "StopFailed"

	ReasonDeleting     Reason = "Deleting"
	ReasonDeleted      Reason = "Deleted"
	ReasonDeleteFailed Reason = "FailedFailed"
)

type (
	Type   string
	Reason string
)

type Report struct {
	report *typesv1.ConditionReport
}

func NewReport(resourceID, reporter string, gen int64) *Report {
	return &Report{
		report: &typesv1.ConditionReport{
			ResourceId:         resourceID,
			Reporter:           reporter,
			ObservedGeneration: gen,
		},
	}
}

func NewReportFor(task *tasksv1.Task, reporter string) *Report {
	return NewReport(task.GetMeta().GetName(), reporter, int64(task.GetMeta().GetRevision()))
}

func (r *Report) Type(t Type) *Report {
	r.report.Type = string(t)
	r.report.ObservedAt = timestamppb.Now()
	return r
}

func (r *Report) True(reason Reason, msg string) *typesv1.ConditionReport {
	r.report.Status = typesv1.ConditionStatus_CONDITION_STATUS_TRUE
	r.report.Reason = string(reason)
	r.report.Msg = string(msg)
	return r.report
}

func (r *Report) False(reason Reason, msg string) *typesv1.ConditionReport {
	r.report.Status = typesv1.ConditionStatus_CONDITION_STATUS_FALSE
	r.report.Reason = string(reason)
	r.report.Msg = string(msg)
	return r.report
}

func (r *Report) InProgress(reason Reason, msg string) *typesv1.ConditionReport {
	r.report.Status = typesv1.ConditionStatus_CONDITION_STATUS_UNKNOWN
	r.report.Reason = string(reason)
	r.report.Msg = string(msg)
	return r.report
}

// func (r *Report) WithMetadata(m map[string]string) *typesv1.ConditionReport {
// 	HHH
// }
//
// .WithMetadata(map[string]string{
//                 "exit_code": "137",
//                 "signal": "SIGKILL",
//                 "oom_killed": "true",
//             }))
