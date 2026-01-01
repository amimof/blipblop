// Package consts provides consts used throughout the system
package consts

const (
	ERRIMAGEPULL    = "ErrPulling"
	ERREESCHEDULING = "ErrScheduling"
	ERREXEC         = "ErrExec"
	ERRDELETE       = "ErrDeleting"
	ERRSTOP         = "ErrStopping"
	ERRKILL         = "ErrKilling"
	ERRPROVISIONING = "ErrProvisioning"
	ERRUPGRADING    = "ErrUpgrading"

	PHASERUNNING     = "running"
	PHASESTOPPED     = "stopped"
	PHASESCHEDULED   = "scheduled"
	PHASEREADY       = "ready"
	PHASEMISSING     = "missing"
	PHASEUNKNOWN     = "unknown"
	PHASEPROVISIONED = "provisioned"
	PHASEPDETACHED   = "detached"
	PHASEATTACHED    = "attached"

	PHASEUPGRADING   = "upgrading"
	PHASEDOWNLOADING = "downloading"
)
