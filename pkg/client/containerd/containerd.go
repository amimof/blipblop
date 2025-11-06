package containerd

import (
	"fmt"
	"log"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
)

func ConnectContainerd(address string) (*containerd.Client, error) {
	client, err := containerd.New(address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to containerd: %w", err)
	}
	return client, nil
}

func ReconnectWithBackoff(address string) (*containerd.Client, error) {
	var (
		client *containerd.Client
		err    error
	)

	backoff := time.Second
	for {
		client, err = ConnectContainerd(address)
		if err == nil {
			return client, nil
		}

		log.Printf("reconnect failed: %v, retrying in %v", err, backoff)
		time.Sleep(backoff)

		backoff = time.Duration(float64(backoff) * 1.5)
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}
