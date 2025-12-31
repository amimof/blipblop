package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	nodesv1 "github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/cmdutil"
	"github.com/amimof/blipblop/pkg/consts"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type NewNodeUpgradeControllerOption func(c *NodeUpgradeController)

func WithNodeUpgradeVersion(version, commit, branch, goversion string) NewNodeUpgradeControllerOption {
	return func(c *NodeUpgradeController) {
		c.nodeVersion = &nodeVersion{
			version:   version,
			commit:    commit,
			branch:    branch,
			goversion: goversion,
		}
	}
}

func WithNodeUpgradeControllerLogger(l logger.Logger) NewNodeUpgradeControllerOption {
	return func(c *NodeUpgradeController) {
		c.logger = l
	}
}

func WithNodeUpgradeControllerDownloadPath(p string) NewNodeUpgradeControllerOption {
	return func(c *NodeUpgradeController) {
		c.targetPath = p
	}
}

func WithNodeUpgradeControllerHttpClient(cl *http.Client) NewNodeUpgradeControllerOption {
	return func(c *NodeUpgradeController) {
		c.httpClient = cl
	}
}

type nodeVersion struct {
	version   string
	commit    string
	branch    string
	goversion string
}

type NodeUpgradeController struct {
	node        *nodesv1.Node
	clientset   *client.ClientSet
	logger      logger.Logger
	nodeVersion *nodeVersion
	tracer      trace.Tracer
	targetPath  string
	tmpPath     string
	binPath     string
	httpClient  *http.Client
}

func (c *NodeUpgradeController) handleErrors(h events.HandlerFunc) events.HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		err := h(ctx, ev)
		if err != nil {
			c.logger.Error("handler returned error", "event", ev.GetType().String(), "error", err)
			return err
		}
		return nil
	}
}

func (c *NodeUpgradeController) Run(ctx context.Context) {
	// Subscribe to events
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_controller_name", "nodeupgrade")
	evt, errCh := c.clientset.EventV1().Subscribe(ctx, events.NodeUpgrade)

	// Setup Node Handlers
	c.clientset.EventV1().On(events.NodeUpgrade, c.handleErrors(c.onNodeUpgrade))

	nodeName := c.node.GetMeta().GetName()

	go func() {
		for e := range evt {
			c.logger.Info("nodeupgrade controller got event", "event", e.GetType().String(), "clientID", nodeName, "objectID", e.GetObjectId())
		}
	}()

	c.logger.Debug("node has version", "version", c.nodeVersion.version, "commit", c.nodeVersion.commit, "branch", c.nodeVersion.branch, "goversion", c.nodeVersion.goversion)

	// Update status once connected
	err := c.clientset.NodeV1().Status().Update(
		ctx,
		nodeName, &nodesv1.Status{
			Version: wrapperspb.String(c.nodeVersion.version),
		},
		"version",
	)
	if err != nil {
		c.logger.Error("error setting node state", "error", err)
	}

	c.binPath, err = os.Executable()
	if err != nil {
		c.logger.Error("error getting executable name from environment", "error", err)
	}

	// Handle errors
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if e != nil {
				c.logger.Error("received error on channel", "error", e)
			}
		}
	}
}

type apiResponse struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (c *NodeUpgradeController) getDownloadURL(ar apiResponse, arch string) (string, error) {
	if ar.Assets == nil {
		return "", fmt.Errorf("no assets in api response")
	}

	allowedArchs := map[string]struct{}{
		"arm":   {},
		"arm64": {},
		"amd64": {},
	}

	if _, ok := allowedArchs[arch]; !ok {
		return "", fmt.Errorf("unsupported architecture %s", arch)
	}

	match := fmt.Sprintf("blipblop-node-linux-%s", arch)
	var res string
	for _, asset := range ar.Assets {
		if asset.Name == match {
			res = asset.DownloadURL
			break
		}
	}

	if res == "" {
		return "", fmt.Errorf("couldn't find a release for %s", match)
	}

	return res, nil
}

func (c *NodeUpgradeController) downloadBinary(ctx context.Context, ver, arch string) (string, error) {
	// Download new node binary
	tmpFile, err := os.CreateTemp(c.tmpPath, "blipblop-node-")
	if err != nil {
		c.failUpgrade(ctx, err)
		return "", err
	}

	defer tmpFile.Close()

	url := fmt.Sprintf("https://api.github.com/repos/amimof/blipblop/releases/tags/%s", ver)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		c.failUpgrade(ctx, err)
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("unexpected response from server: %s", resp.Status)
		c.failUpgrade(ctx, err)
		return "", err
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		c.failUpgrade(ctx, err)
		return "", err
	}

	var ar apiResponse
	err = json.Unmarshal(b, &ar)
	if err != nil {
		c.failUpgrade(ctx, err)
		return "", err
	}

	nodeName := c.node.GetMeta().GetName()

	assetURL, err := c.getDownloadURL(ar, arch)
	if err != nil {
		c.failUpgrade(ctx, err)
		return "", err
	}

	err = c.clientset.NodeV1().Status().Update(
		ctx,
		nodeName, &nodesv1.Status{
			Phase: wrapperspb.String(consts.PHASEDOWNLOADING),
		},
		"phase", "status",
	)
	if err != nil {
		c.failUpgrade(ctx, err)
		return "", err
	}

	binResp, err := c.httpClient.Get(assetURL)
	if err != nil {
		c.failUpgrade(ctx, err)
		return "", err
	}

	defer binResp.Body.Close()

	written, err := io.Copy(tmpFile, binResp.Body)
	if err != nil {
		c.failUpgrade(ctx, err)
		return "", err
	}

	c.logger.Debug("downloaded node binary", "path", tmpFile.Name(), "target_version", ver, "previous_version", c.nodeVersion.version, "size_bytes", written, "asset_url", assetURL)

	return tmpFile.Name(), nil
}

func (c *NodeUpgradeController) replaceBinary(src, dst string) error {
	// Validate input
	if src == "" {
		return fmt.Errorf("source file cannot be empty")
	}
	if dst == "" {
		return fmt.Errorf("source file cannot be empty")
	}

	backupPath := fmt.Sprintf("%s_backup", dst)
	backupFile, err := os.Create(backupPath)
	if err != nil {
		return err
	}

	defer backupFile.Close()

	dstFile, err := os.Open(dst)
	if err != nil {
		return err
	}

	defer dstFile.Close()

	// Copy existing binary to a backup file with the leading _backup
	_, err = io.Copy(backupFile, dstFile)
	if err != nil {
		return err
	}

	err = os.Rename(src, dst)
	if err != nil {
		return err
	}

	return nil
}

func (c *NodeUpgradeController) onNodeUpgrade(ctx context.Context, e *eventsv1.Event) error {
	_, span := c.tracer.Start(ctx, "controller.nodeupgrade.OnNodeUpgrade")
	defer span.End()

	var req nodesv1.UpgradeRequest
	err := e.GetObject().UnmarshalTo(&req)
	if err != nil {
		c.logger.Debug("error unmarshaling request from event", "error", err)
		return err
	}

	c.logger.Debug("received node upgrade request", "target_version", req.GetTargetVersion(), "current_version", c.nodeVersion.version)

	nodeName := c.node.GetMeta().GetName()

	// Update status as soon as upgrade begins
	err = c.clientset.NodeV1().Status().Update(
		ctx,
		nodeName, &nodesv1.Status{
			Phase: wrapperspb.String(consts.PHASEUPGRADING),
		},
		"phase",
	)
	if err != nil {
		c.logger.Error("error setting node state", "error", err)
	}

	// Parse the version from flag
	ver, err := cmdutil.ParseVersion(req.GetTargetVersion())
	if err != nil {
		c.failUpgrade(ctx, err)
		return err
	}

	// Build download URL based on version and architecture
	downloadPath, err := c.downloadBinary(ctx, ver, "amd64")
	if err != nil {
		return err
	}

	// Move binaries in place
	err = c.replaceBinary(downloadPath, c.binPath)
	if err != nil {
		c.failUpgrade(ctx, err)
		return err
	}

	// Update status once connected
	return c.clientset.NodeV1().Status().Update(
		ctx,
		nodeName, &nodesv1.Status{
			Phase:  wrapperspb.String(consts.PHASEREADY),
			Status: wrapperspb.String(""),
		},
		"phase", "status",
	)
}

func (c *NodeUpgradeController) failUpgrade(ctx context.Context, err error) {
	nodeName := c.node.GetMeta().GetName()
	_ = c.clientset.NodeV1().Status().Update(
		ctx,
		nodeName, &nodesv1.Status{
			Phase:  wrapperspb.String(consts.ERRUPGRADING),
			Status: wrapperspb.String(err.Error()),
		},
		"phase", "status",
	)
}

func NewNodeUpgradeController(c *client.ClientSet, n *nodesv1.Node, opts ...NewNodeUpgradeControllerOption) *NodeUpgradeController {
	m := &NodeUpgradeController{
		node:       n,
		clientset:  c,
		logger:     logger.ConsoleLogger{},
		tracer:     otel.Tracer("node-upgrade-controller"),
		targetPath: "/usr/local/bin/",
		tmpPath:    "/tmp/",
	}
	for _, opt := range opts {
		opt(m)
	}

	return m
}
