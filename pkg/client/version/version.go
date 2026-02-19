package version

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	containersetsv1 "github.com/amimof/voiyd/api/services/containersets/v1"
	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	volumesv1 "github.com/amimof/voiyd/api/services/volumes/v1"
)

var apiVersionByFullName = map[protoreflect.FullName]string{
	"services.tasks.v1.Task":                 "task/v1",
	"services.nodes.v1.Node":                 "node/v1",
	"services.volumes.v1.Volume":             "volume/v1",
	"services.events.v1.Event":               "event/v1",
	"services.leases.v1.Lease":               "lease/v1",
	"services.containersets.v1.ContainerSet": "containerset/v1",
}

var (
	VersionTask         = Version((&tasksv1.Task{}))
	VersionNode         = Version((&nodesv1.Node{}))
	VersionVolume       = Version((&volumesv1.Volume{}))
	VersionEvent        = Version((&eventsv1.Event{}))
	VersionLease        = Version((&leasesv1.Lease{}))
	VersionContainerSet = Version((&containersetsv1.ContainerSet{}))
)

func Version(m proto.Message) string {
	return apiVersionByFullName[m.ProtoReflect().Descriptor().FullName()]
}
