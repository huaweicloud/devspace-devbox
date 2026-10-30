package client

import (
	"context"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api"
)

// SandboxListResult carries the sandbox list with pagination meta.
type SandboxListResult struct {
	Sandboxes []api.Sandbox
	NextToken string
}

// ListSandboxes returns all sandboxes, optionally filtered/paginated.
func (c *Client) ListSandboxes(ctx context.Context, params *api.ListSandboxesParams) (*SandboxListResult, error) {
	resp, err := c.api.ListSandboxesWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	return &SandboxListResult{
		Sandboxes: *resp.JSON200,
		NextToken: resp.HTTPResponse.Header.Get(HeaderNextToken),
	}, nil
}

// CreateSandbox creates a sandbox from a template.
func (c *Client) CreateSandbox(ctx context.Context, body api.CreateSandboxJSONRequestBody) (*api.Sandbox, error) {
	resp, err := c.api.CreateSandboxWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	return resp.JSON201, nil
}

// GetSandbox returns full details for a sandbox.
func (c *Client) GetSandbox(ctx context.Context, sandboxID string) (*api.Sandbox, error) {
	resp, err := c.api.GetSandboxWithResponse(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	return resp.JSON200, nil
}

// DeleteSandbox kills a sandbox.
func (c *Client) DeleteSandbox(ctx context.Context, sandboxID string) error {
	resp, err := c.api.DeleteSandboxWithResponse(ctx, sandboxID)
	if err != nil {
		return err
	}
	return checkStatus(resp.HTTPResponse, resp.Body)
}

// PauseSandbox pauses a running sandbox.
func (c *Client) PauseSandbox(ctx context.Context, sandboxID string) error {
	resp, err := c.api.PauseSandboxWithResponse(ctx, sandboxID)
	if err != nil {
		return err
	}
	return checkStatus(resp.HTTPResponse, resp.Body)
}

// ResumeSandbox resumes a paused sandbox.
func (c *Client) ResumeSandbox(ctx context.Context, sandboxID string, body api.ResumeSandboxJSONRequestBody) (*api.Sandbox, error) {
	resp, err := c.api.ResumeSandboxWithResponse(ctx, sandboxID, body)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	return resp.JSON201, nil
}

// ConnectSandbox returns sandbox details, resuming it if paused.
func (c *Client) ConnectSandbox(ctx context.Context, sandboxID string, body api.ConnectSandboxJSONRequestBody) (*api.Sandbox, error) {
	resp, err := c.api.ConnectSandboxWithResponse(ctx, sandboxID, body)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return resp.JSON201, nil
}

// SetSandboxTimeout sets the time-to-live of a sandbox.
func (c *Client) SetSandboxTimeout(ctx context.Context, sandboxID string, body api.SetSandboxTimeoutJSONRequestBody) error {
	resp, err := c.api.SetSandboxTimeoutWithResponse(ctx, sandboxID, body)
	if err != nil {
		return err
	}
	return checkStatus(resp.HTTPResponse, resp.Body)
}

// RefreshSandbox extends the sandbox time-to-live.
func (c *Client) RefreshSandbox(ctx context.Context, sandboxID string, body api.RefreshSandboxJSONRequestBody) error {
	resp, err := c.api.RefreshSandboxWithResponse(ctx, sandboxID, body)
	if err != nil {
		return err
	}
	return checkStatus(resp.HTTPResponse, resp.Body)
}
