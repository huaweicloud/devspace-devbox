package client

import (
	"context"

	"github.com/huaweicloud/devspace-devbox/packages/cli/internal/api"
)

// ListTemplates returns the latest usable build of each public template.
func (c *Client) ListTemplates(ctx context.Context) ([]api.Template, error) {
	resp, err := c.api.ListTemplatesWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return []api.Template{}, nil
	}
	return *resp.JSON200, nil
}
