package conditioncontroller

import (
	"context"
	"fmt"
	"strconv"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/condition"
	"github.com/amimof/voiyd/pkg/consts"
	errs "github.com/amimof/voiyd/pkg/errors"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

type NewOption func(c *Controller)

func WithLogger(l logger.Logger) NewOption {
	return func(c *Controller) {
		c.logger = l
	}
}

func WithExchange(e *events.Exchange) NewOption {
	return func(c *Controller) {
		c.exchange = e
	}
}

type Controller struct {
	clientset *client.ClientSet
	logger    logger.Logger
	exchange  *events.Exchange
}

func (c *Controller) Run(ctx context.Context) {
	// Subscribe to events
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_controller_name", "scheduler")
	_, err := c.clientset.EventV1().Subscribe(ctx, events.ConditionReported)

	// Setup Handlers
	c.clientset.EventV1().On(events.ConditionReported, events.HandleErrors(c.logger, events.HandleConditionReport(c.onConditionReported)))

	// Handle errors
	for e := range err {
		c.logger.Error("received error on channel", "error", e)
	}
}

func (c *Controller) onConditionReported(ctx context.Context, report *typesv1.ConditionReport) error {
	c.logger.Debug("condition controller received a report", "reporter", report.GetReporter(), "resource", report.GetResourceId(), "observedGeneration", report.GetObservedGeneration())

	// 1. Validate the report
	resourceID := report.GetResourceId()
	observedGen := report.GetObservedGeneration()

	// 2. Get the current Task
	task, err := c.clientset.TaskV1().Get(ctx, resourceID)
	if err != nil {
		if errs.IsNotFound(err) {
			c.logger.Warn("condition report for non-existent task", "task", resourceID)
			return nil // Don't fail on deleted resources
		}
		return err
	}

	// 3. Validate generation (prevent stale updates)
	currentGen := int64(task.GetMeta().GetRevision())
	if observedGen != 0 && observedGen < currentGen {
		c.logger.Warn("skipping stale condition report",
			"task", resourceID,
			"observedGen", observedGen,
			"currentGen", currentGen)
		return nil
	}

	// 4. Convert ConditionReport to Condition
	newCondition := reportToCondition(report, task)

	fmt.Printf("%+v\n", newCondition)

	// Get node id from report
	nodeID := getNodeFromReport(report)

	// Get pid from report
	pid := getPidFromReport(report)
	id := getIDFromReport(report)

	// 5. Merge with existing conditions
	updatedConditions := mergeCondition(task.GetStatus().GetConditions(), newCondition)

	phase := getPhaseFromConditions(updatedConditions)

	// 6. Update Task status
	return c.clientset.TaskV1().Status().Update(
		ctx,
		resourceID,
		&tasksv1.Status{
			Conditions: updatedConditions,
			Phase:      wrapperspb.String(phase),
			Node:       nodeID,
			Id:         id,
			Pid:        pid,
		},
		"conditions", "phase", "node", "id", "pid",
	)
}

func getNodeFromReport(report *typesv1.ConditionReport) *wrapperspb.StringValue {
	if condition.Type(report.GetType()) == condition.Type(condition.TaskScheduled) {
		if v, ok := report.GetMetadata()["node"]; ok {
			return wrapperspb.String(v)
		}
	}
	return nil
}

func getIDFromReport(report *typesv1.ConditionReport) *wrapperspb.StringValue {
	if condition.Type(report.GetType()) == condition.Type(condition.TaskReady) {
		if v, ok := report.GetMetadata()["id"]; ok {
			return wrapperspb.String(v)
		}
	}
	return nil
}

func getPidFromReport(report *typesv1.ConditionReport) *wrapperspb.UInt32Value {
	if condition.Type(report.GetType()) == condition.Type(condition.TaskReady) {
		if v, ok := report.GetMetadata()["pid"]; ok {
			if i, err := strconv.Atoi(v); err == nil {
				return wrapperspb.UInt32(uint32(i))
			}
		}
	}
	return nil
}

func getPhaseFromConditions(conds []*typesv1.Condition) string {
	if len(conds) == 0 {
		return consts.PHASEUNKNOWN
	}

	// Find most recently transitioned condition
	var mostRecent *typesv1.Condition
	for _, c := range conds {
		if mostRecent == nil ||
			c.GetLastTransitionTime().AsTime().After(mostRecent.GetLastTransitionTime().AsTime()) {
			mostRecent = c
		}
	}

	// Derive phase from most recent condition
	return phaseFromCondition(mostRecent)
}

func phaseFromCondition(c *typesv1.Condition) string {
	reason := condition.Reason(c.GetReason())

	// Map condition reason to phase
	reasonToPhase := map[condition.Reason]string{
		condition.ReasonScheduled:   consts.PHASESCHEDULING,
		condition.ReasonPulling:     consts.PHASEPULLING,
		condition.ReasonStarting:    consts.PHASESTARTING,
		condition.ReasonRunning:     consts.PHASERUNNING,
		condition.ReasonStopping:    consts.PHASESTOPPING,
		condition.ReasonStopped:     consts.PHASESTOPPED,
		condition.ReasonDeleting:    consts.PHASEDELETING,
		condition.ReasonPullFailed:  consts.ERRIMAGEPULL,
		condition.ReasonStartFailed: consts.ERREXEC,

		condition.ReasonDetached:  "detached",
		condition.ReasonDetaching: "detaching",

		condition.ReasonDetachFailed: "ErrDetaching",
		condition.ReasonAttached:     "attached",
		condition.ReasonAttaching:    "attaching",
		condition.ReasonAttachFailed: "ErrAttaching",
	}

	if phase, ok := reasonToPhase[reason]; ok {
		return phase
	}

	return consts.PHASEUNKNOWN
}

func reportToCondition(report *typesv1.ConditionReport, task *tasksv1.Task) *typesv1.Condition {
	// Find existing condition with same type to preserve last_transition_time if needed
	var existingCondition *typesv1.Condition
	for _, cond := range task.GetStatus().GetConditions() {
		if cond.GetType() == report.GetType() {
			existingCondition = cond
			break
		}
	}

	// Determine if status changed
	statusChanged := existingCondition == nil ||
		existingCondition.GetStatus() != conditionStatusToBoolValue(report.GetStatus())

	// Use existing timestamp if status hasn't changed, otherwise use now
	transitionTime := report.GetObservedAt()
	if !statusChanged && existingCondition != nil {
		transitionTime = existingCondition.GetLastTransitionTime()
	}

	return &typesv1.Condition{
		Type:               report.GetType(),
		Status:             conditionStatusToBoolValue(report.GetStatus()),
		Reason:             report.GetReason(),
		Msg:                report.GetMsg(),
		LastTransitionTime: transitionTime,
	}
}

func conditionStatusToBoolValue(status typesv1.ConditionStatus) *wrapperspb.BoolValue {
	switch status {
	case typesv1.ConditionStatus_CONDITION_STATUS_TRUE:
		return wrapperspb.Bool(true)
	default:
		return wrapperspb.Bool(false)
	}
}

func mergeCondition(existing []*typesv1.Condition, new *typesv1.Condition) []*typesv1.Condition {
	// Clone existing to avoid mutations
	result := make([]*typesv1.Condition, 0, len(existing)+1)

	found := false
	for _, cond := range existing {
		if cond.GetType() == new.GetType() {
			// Replace with new condition
			result = append(result, new)
			found = true
		} else {
			// Keep other conditions unchanged
			result = append(result, cond)
		}
	}

	// If condition type doesn't exist, append it
	if !found {
		result = append(result, new)
	}

	return result
}

func New(cs *client.ClientSet, opts ...NewOption) *Controller {
	c := &Controller{
		clientset: cs,
		logger:    logger.ConsoleLogger{},
	}
	for _, opt := range opts {
		opt(c)
	}

	return c
}
