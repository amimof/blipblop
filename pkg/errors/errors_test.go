package errors

import (
	"context"
	"testing"

	"github.com/amimof/voiyd/pkg/keys"
	"github.com/amimof/voiyd/pkg/repository"
	bd "github.com/amimof/voiyd/pkg/repository/badger"
	"github.com/amimof/voiyd/pkg/repository/inmemory"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
)

func Test_Errors(t *testing.T) {
	db, err := badger.Open(badger.DefaultOptions("/tmp/badger-test"))
	if err != nil {
		t.Fatal(err)
	}

	leaseStore := repository.NewLeaseRepo(inmemory.New())
	leaseStoreB := repository.NewLeaseRepo(bd.New(db))

	tests := []struct {
		name   string
		expect error
		input  func() error
	}{
		{
			name:   "should return not found error",
			expect: repository.ErrNotFound,
			input: func() error {
				uid, err := keys.Name("non-existent-lease")
				if err != nil {
					return err
				}
				_, err = leaseStore.Get(context.Background(), uid)
				return err
			},
		},
		{
			name:   "should return not found error",
			expect: repository.ErrNotFound,
			input: func() error {
				uid, err := keys.Name("non-existent-lease")
				if err != nil {
					return err
				}
				_, err = leaseStoreB.Get(context.Background(), uid)
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
