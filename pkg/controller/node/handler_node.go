package nodecontroller

import (
	"context"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	"go.opentelemetry.io/otel/attribute"
)

func (c *Controller) onNodeConnect(ctx context.Context, e *eventsv1.Event) error {
	_, span := c.tracer.Start(ctx, "controller.node.OnNodeConnect")
	defer span.End()

	var n nodesv1.Node
	err := e.GetObject().UnmarshalTo(&n)
	if err != nil {
		return err
	}

	span.SetAttributes(
		attribute.String("client.id", e.GetClientId()),
		attribute.String("object.id", e.GetObjectId()),
		attribute.String("node.id", n.GetMeta().GetName()),
	)

	return nil
}

func (c *Controller) onNodeDelete(ctx context.Context, obj *eventsv1.Event) error {
	if obj.GetMeta().GetName() != c.node.GetMeta().GetName() {
		return nil
	}
	err := c.clientset.NodeV1().Forget(ctx, obj.GetMeta().GetName())
	if err != nil {
		c.logger.Error("error unjoining node", "node", obj.GetMeta().GetName(), "error", err)
		return err
	}
	c.logger.Debug("successfully unjoined node", "node", obj.GetMeta().GetName())
	return nil
}
