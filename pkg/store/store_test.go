package store

import (
	"log"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/amimof/voiyd/api/types/v1"
	"github.com/amimof/voiyd/services/task"
	"github.com/stretchr/testify/assert"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

var tasks []*tasksv1.Task = []*tasksv1.Task{
	{
		Version: task.Version,
		Meta: &types.Meta{
			Name: "task-1234",
		},
		Config: &tasksv1.Config{
			Image: "docker.io/library/nginx:latest",
			PortMappings: []*tasksv1.PortMapping{
				{
					Name:       "http",
					TargetPort: 8080,
					HostPort:   8080,
					Protocol:   "TCP",
				},
			},
		},
	},
}

func Test_Store_Ephemeral(t *testing.T) {
	store := NewEphemeralStore()

	for _, t := range tasks {
		if err := store.Save(t.GetMeta().GetName(), t); err != nil {
			log.Fatal(err)
		}
	}

	tests := []struct {
		name   string
		id     string
		input  proto.Message
		expect proto.Message
	}{
		{
			id:   "task-1234",
			name: "should be identical",
			input: &tasksv1.Task{
				Version: task.Version,
				Meta: &types.Meta{
					Name: "task-124",
				},
				Config: &tasksv1.Config{
					Image: "docker.io/library/nginx:latest",
					PortMappings: []*tasksv1.PortMapping{
						{
							Name:       "http",
							TargetPort: 8080,
							HostPort:   8080,
							Protocol:   "TCP",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var loaded tasksv1.Task
			err := store.Load(test.id, &loaded)
			if err != nil {
				log.Fatal(err)
			}

			if !proto.Equal(&loaded, tasks[0]) {
				log.Printf("Got %v\n", &loaded)
				log.Printf("expected %v\n", tasks[0])
				assert.Fail(t, "loaded message didn't match expectation")
			}
		})
	}
}
