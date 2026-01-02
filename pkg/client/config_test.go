package client_test

import (
	"testing"

	"github.com/amimof/blipblop/pkg/client"
)

func TestConfig_ValidateTLSConfig(t *testing.T) {
	c := &client.Config{
		Current: "prod",
		Servers: []*client.Server{
			{
				Name:    "prod",
				Address: "localhost:5700",
			},
		},
	}

	curr, err := c.CurrentServer()
	if err != nil {
		t.Fatal(err)
	}
	err = curr.TLSConfig.Validate()
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: construct the receiver type.
			var c client.Config
			gotErr := c.Validate()
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("validate() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("validate() succeeded unexpectedly")
			}
		})
	}
}
