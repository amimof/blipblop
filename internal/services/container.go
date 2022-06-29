package services

import (
	"context"
	"github.com/amimof/blipblop/api/services/containers/v1"
)

var containerService *ContainerService

type ContainerService struct {
	containers.UnimplementedContainerServiceServer
}

func (c *ContainerService) GetTest(ctx context.Context, container *containers.GetContainerRequest) (*containers.GetContainerResponse, error) {
	return &containers.GetContainerResponse{
		Container: &containers.Container{
			Image: "docker.io/amimo/node-cert-exporter:latest",
		},
	}, nil
}

func (c *ContainerService) CreateTest(ctx context.Context, container *containers.CreateContainerRequest) (*containers.CreateContainerResponse, error) {
	return &containers.CreateContainerResponse{
		Container: container.Container,
	}, nil
}

func newContainerService() *ContainerService {
	return &ContainerService{}
}

func NewContainerService() *ContainerService {
	if containerService == nil {
		containerService = newContainerService()
	}
	return containerService
}
