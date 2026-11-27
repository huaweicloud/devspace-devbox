/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

package client

import (
	"context"
	"fmt"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api/rest/manager"
	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
)

// SandboxListResult carries the sandbox list with pagination meta.
type SandboxListResult struct {
	Sandboxes []manager.Sandbox
	NextToken string
}

// ListSandboxes returns all sandboxes, optionally filtered/paginated.
func (c *Client) ListSandboxes(ctx context.Context, params *manager.ListSandboxesParams) (*SandboxListResult, error) {
	resp, err := c.manager.ListSandboxesWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("%s", i18n.T(i18n.Msg.Client.EmptyResponseBody))
	}
	return &SandboxListResult{
		Sandboxes: *resp.JSON200,
		NextToken: resp.HTTPResponse.Header.Get(HeaderNextToken),
	}, nil
}

// CreateSandbox creates a sandbox from a template.
func (c *Client) CreateSandbox(
	ctx context.Context,
	body manager.CreateSandboxJSONRequestBody,
) (*manager.Sandbox, error) {
	resp, err := c.manager.CreateSandboxWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, fmt.Errorf("%s", i18n.T(i18n.Msg.Client.EmptyResponseBody))
	}
	return resp.JSON201, nil
}

// GetSandbox returns full details for a sandbox.
func (c *Client) GetSandbox(ctx context.Context, sandboxID string) (*manager.Sandbox, error) {
	resp, err := c.manager.GetSandboxWithResponse(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("%s", i18n.T(i18n.Msg.Client.EmptyResponseBody))
	}
	return resp.JSON200, nil
}

// GetSandboxMetrics returns the historical metrics series of a sandbox.
func (c *Client) GetSandboxMetrics(ctx context.Context, sandboxID string, params *manager.GetSandboxMetricsParams) ([]manager.SandboxMetricsPoint, error) {
	resp, err := c.manager.GetSandboxMetricsWithResponse(ctx, sandboxID, params)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("%s", i18n.T(i18n.Msg.Client.EmptyResponseBody))
	}
	return *resp.JSON200, nil
}

// DeleteSandbox kills a sandbox.
func (c *Client) DeleteSandbox(ctx context.Context, sandboxID string) error {
	resp, err := c.manager.DeleteSandboxWithResponse(ctx, sandboxID)
	if err != nil {
		return err
	}
	return checkStatus(resp.HTTPResponse, resp.Body)
}

// PauseSandbox pauses a running sandbox.
func (c *Client) PauseSandbox(ctx context.Context, sandboxID string) error {
	resp, err := c.manager.PauseSandboxWithResponse(ctx, sandboxID)
	if err != nil {
		return err
	}
	return checkStatus(resp.HTTPResponse, resp.Body)
}

// ResumeSandbox resumes a paused sandbox.
func (c *Client) ResumeSandbox(
	ctx context.Context,
	sandboxID string,
	body manager.ResumeSandboxJSONRequestBody,
) (*manager.Sandbox, error) {
	resp, err := c.manager.ResumeSandboxWithResponse(ctx, sandboxID, body)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, fmt.Errorf("%s", i18n.T(i18n.Msg.Client.EmptyResponseBody))
	}
	return resp.JSON201, nil
}

// ConnectSandbox returns sandbox details, resuming it if paused.
func (c *Client) ConnectSandbox(
	ctx context.Context,
	sandboxID string,
	body manager.ConnectSandboxJSONRequestBody,
) (*manager.Sandbox, error) {
	resp, err := c.manager.ConnectSandboxWithResponse(ctx, sandboxID, body)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	if resp.JSON201 == nil {
		return nil, fmt.Errorf("%s", i18n.T(i18n.Msg.Client.EmptyResponseBody))
	}
	return resp.JSON201, nil
}

// SetSandboxTimeout sets the time-to-live of a sandbox.
func (c *Client) SetSandboxTimeout(
	ctx context.Context,
	sandboxID string,
	body manager.SetSandboxTimeoutJSONRequestBody,
) error {
	resp, err := c.manager.SetSandboxTimeoutWithResponse(ctx, sandboxID, body)
	if err != nil {
		return err
	}
	return checkStatus(resp.HTTPResponse, resp.Body)
}

// RefreshSandbox extends the sandbox time-to-live.
func (c *Client) RefreshSandbox(
	ctx context.Context,
	sandboxID string,
	body manager.RefreshSandboxJSONRequestBody,
) error {
	resp, err := c.manager.RefreshSandboxWithResponse(ctx, sandboxID, body)
	if err != nil {
		return err
	}
	return checkStatus(resp.HTTPResponse, resp.Body)
}
