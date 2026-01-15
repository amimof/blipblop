package lease

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	"github.com/stretchr/testify/assert"
)

func Test_Expirty(t *testing.T) {
	lease := &leasesv1.Lease{
		TaskId:     "nginx",
		NodeId:     "node-01",
		AcquiredAt: timestamppb.Now(),
		RenewTime:  timestamppb.Now(),
		ExpiresAt:  timestamppb.New(time.Now().Add(time.Duration(30) * time.Second)),
		TtlSeconds: 30,
	}

	notExpired := time.Now().Before(lease.GetExpiresAt().AsTime())
	assert.True(t, notExpired, "should be true")
}
