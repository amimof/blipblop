package errors

import (
	"context"
	"testing"

	"github.com/amimof/voiyd/pkg/repository"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
)

func Test_Errors(t *testing.T) {
	tests := []struct {
		name   string
		expect error
		input  func() error
	}{
		{
			name:   "should return not found error",
			expect: repository.ErrNotFound,
			input: func() error {
				leaseStore := repository.NewLeaseInMemRepo()
				_, err := leaseStore.Get(context.Background(), "non-existent-lease")
				return err
			},
		},
		{
			name:   "should return not found error",
			expect: repository.ErrNotFound,
			input: func() error {
				db, err := badger.Open(badger.DefaultOptions("/tmp/badger-test"))
				if err != nil {
					return err
				}
				leaseStore := repository.NewNodeBadgerRepository(db)
				_, err = leaseStore.Get(context.Background(), "non-existent-lease")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.ErrorIs(t, test.input(), test.expect, test.name)
		})
	}
}
