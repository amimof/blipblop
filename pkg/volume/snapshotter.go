package volume

import "context"

type Snapshotter interface {
	Snapshot(ctx context.Context, vol *Volume) error
}
