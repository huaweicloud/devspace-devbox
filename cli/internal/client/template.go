/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

package client

import (
	"context"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api/rest/manager"
)

// ListTemplates returns the latest usable build of each public template.
func (c *Client) ListTemplates(ctx context.Context) ([]manager.Template, error) {
	resp, err := c.manager.ListTemplatesWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return []manager.Template{}, nil
	}
	return *resp.JSON200, nil
}
