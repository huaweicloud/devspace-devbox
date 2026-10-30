// Package api provides primitives to interact with the openapi HTTP API.
//
// Typed client for the DevBox Manager API (originally generated from openapi.yaml by oapi-codegen, now hand-maintained).
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/oapi-codegen/runtime"
)

// Defines values for SandboxLifecycleOnTimeout.
const (
	Kill  SandboxLifecycleOnTimeout = "kill"
	Pause SandboxLifecycleOnTimeout = "pause"
)

// Valid indicates whether the value is a known member of the SandboxLifecycleOnTimeout enum.
func (e SandboxLifecycleOnTimeout) Valid() bool {
	switch e {
	case Kill:
		return true
	case Pause:
		return true
	default:
		return false
	}
}

// Defines values for SandboxState.
const (
	Paused  SandboxState = "paused"
	Running SandboxState = "running"
)

// Valid indicates whether the value is a known member of the SandboxState enum.
func (e SandboxState) Valid() bool {
	switch e {
	case Paused:
		return true
	case Running:
		return true
	default:
		return false
	}
}

// Error defines model for Error.
type Error struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// NewSandbox defines model for NewSandbox.
type NewSandbox struct {
	// BuildID Optional pinned build; defaults to the latest build of the selected template.
	BuildID    *string            `json:"buildID,omitempty"`
	EnvVars    *map[string]string `json:"envVars,omitempty"`
	Lifecycle  *SandboxLifecycle  `json:"lifecycle,omitempty"`
	Metadata   *map[string]string `json:"metadata,omitempty"`
	TemplateID string             `json:"templateID"`
	Timeout    *int32             `json:"timeout,omitempty"`
}

// Sandbox defines model for Sandbox.
type Sandbox struct {
	ConnectToken *string `json:"connectToken,omitempty"`
	CpuCount     int32   `json:"cpuCount"`
	DiskSizeMB   int32   `json:"diskSizeMB"`

	// Domain Complete EnvD HTTPS endpoint for this sandbox. Paused sandboxes do not include connection credentials.
	Domain *string `json:"domain,omitempty"`

	// EndAt Running deadline. Null while paused; paused sandboxes have no Manager retention deadline.
	EndAt           *time.Time         `json:"endAt"`
	Lifecycle       SandboxLifecycle   `json:"lifecycle"`
	MemoryMB        int32              `json:"memoryMB"`
	Metadata        *map[string]string `json:"metadata,omitempty"`
	SandboxID       string             `json:"sandboxID"`
	StartedAt       time.Time          `json:"startedAt"`
	State           SandboxState       `json:"state"`
	TemplateID      string             `json:"templateID"`
	TokenExpiration *int64             `json:"tokenExpiration,omitempty"`
	TunnelId        *string            `json:"tunnelId,omitempty"`
}

// SandboxLifecycle defines model for SandboxLifecycle.
type SandboxLifecycle struct {
	AutoResume bool                      `json:"autoResume"`
	OnTimeout  SandboxLifecycleOnTimeout `json:"onTimeout"`
}

// SandboxLifecycleOnTimeout defines model for SandboxLifecycle.OnTimeout.
type SandboxLifecycleOnTimeout string

// SandboxState defines model for SandboxState.
type SandboxState string

// Template defines model for Template.
type Template struct {
	Aliases    []string `json:"aliases"`
	BuildID    string   `json:"buildID"`
	CpuCount   int32    `json:"cpuCount"`
	DiskSizeMB int32    `json:"diskSizeMB"`
	MemoryMB   int32    `json:"memoryMB"`
	Name       string   `json:"name"`
	TemplateID string   `json:"templateID"`
}

// TimeoutOption defines model for TimeoutOption.
type TimeoutOption struct {
	Timeout *int32 `json:"timeout,omitempty"`
}

// SandboxID defines model for SandboxID.
type SandboxID = string

// BadRequest defines model for BadRequest.
type BadRequest = Error

// Conflict defines model for Conflict.
type Conflict = Error

// Forbidden defines model for Forbidden.
type Forbidden = Error

// NotFound defines model for NotFound.
type NotFound = Error

// RateLimited defines model for RateLimited.
type RateLimited = Error

// Unauthorized defines model for Unauthorized.
type Unauthorized = Error

// Unavailable defines model for Unavailable.
type Unavailable = Error

// ListSandboxesParams defines parameters for ListSandboxes.
type ListSandboxesParams struct {
	Metadata  *string         `form:"metadata,omitempty" json:"metadata,omitempty"`
	State     *[]SandboxState `form:"state,omitempty" json:"state,omitempty"`
	Template  *string         `form:"template,omitempty" json:"template,omitempty"`
	NextToken *string         `form:"nextToken,omitempty" json:"nextToken,omitempty"`
	Limit     *int            `form:"limit,omitempty" json:"limit,omitempty"`
}

// RefreshSandboxJSONBody defines parameters for RefreshSandbox.
type RefreshSandboxJSONBody struct {
	Duration int32 `json:"duration"`
}

// CreateSandboxJSONRequestBody defines body for CreateSandbox for application/json ContentType.
type CreateSandboxJSONRequestBody = NewSandbox

// ConnectSandboxJSONRequestBody defines body for ConnectSandbox for application/json ContentType.
type ConnectSandboxJSONRequestBody = TimeoutOption

// RefreshSandboxJSONRequestBody defines body for RefreshSandbox for application/json ContentType.
type RefreshSandboxJSONRequestBody RefreshSandboxJSONBody

// ResumeSandboxJSONRequestBody defines body for ResumeSandbox for application/json ContentType.
type ResumeSandboxJSONRequestBody = TimeoutOption

// SetSandboxTimeoutJSONRequestBody defines body for SetSandboxTimeout for application/json ContentType.
type SetSandboxTimeoutJSONRequestBody = TimeoutOption

// RequestEditorFn is the function signature for the RequestEditor callback function
type RequestEditorFn func(ctx context.Context, req *http.Request) error

// Doer performs HTTP requests.
//
// The standard http.Client implements this interface.
type HttpRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client which conforms to the OpenAPI3 specification for this service.
type Client struct {
	// The endpoint of the server conforming to this interface, with scheme,
	// https://api.deepmap.com for example. This can contain a path relative
	// to the server, such as https://api.deepmap.com/dev-test, and all the
	// paths in the swagger spec will be appended to the server.
	Server string

	// Doer for performing requests, typically a *http.Client with any
	// customized settings, such as certificate chains.
	Client HttpRequestDoer

	// A list of callbacks for modifying requests which are generated before sending over
	// the network.
	RequestEditors []RequestEditorFn
}

// ClientOption allows setting custom parameters during construction
type ClientOption func(*Client) error

// Creates a new Client, with reasonable defaults
func NewClient(server string, opts ...ClientOption) (*Client, error) {
	// create a client with sane default values
	client := Client{
		Server: server,
	}
	// mutate client and add all optional params
	for _, o := range opts {
		if err := o(&client); err != nil {
			return nil, err
		}
	}
	// ensure the server URL always has a trailing slash
	if !strings.HasSuffix(client.Server, "/") {
		client.Server += "/"
	}
	// create httpClient, if not already present
	if client.Client == nil {
		client.Client = &http.Client{}
	}
	return &client, nil
}

// WithHTTPClient allows overriding the default Doer, which is
// automatically created using http.Client. This is useful for tests.
func WithHTTPClient(doer HttpRequestDoer) ClientOption {
	return func(c *Client) error {
		c.Client = doer
		return nil
	}
}

// WithRequestEditorFn allows setting up a callback function, which will be
// called right before sending the request. This can be used to mutate the request.
func WithRequestEditorFn(fn RequestEditorFn) ClientOption {
	return func(c *Client) error {
		c.RequestEditors = append(c.RequestEditors, fn)
		return nil
	}
}

// The interface specification for the client above.
type ClientInterface interface {

	// ListSandboxes performs a GET /sandboxes (the `ListSandboxes` operationId) request.
	ListSandboxes(ctx context.Context, params *ListSandboxesParams, reqEditors ...RequestEditorFn) (*http.Response, error)

	// CreateSandboxWithBody performs a POST /sandboxes (the `CreateSandbox` operationId) request,
	// with any type of body and a specified content type.
	CreateSandboxWithBody(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error)

	// CreateSandbox performs a POST /sandboxes (the `CreateSandbox` operationId) request.
	// Takes a body of the `application/json` content type.
	CreateSandbox(ctx context.Context, body CreateSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error)

	// DeleteSandbox performs a DELETE /sandboxes/{sandboxID} (the `DeleteSandbox` operationId) request.
	DeleteSandbox(ctx context.Context, sandboxID SandboxID, reqEditors ...RequestEditorFn) (*http.Response, error)

	// GetSandbox performs a GET /sandboxes/{sandboxID} (the `GetSandbox` operationId) request.
	GetSandbox(ctx context.Context, sandboxID SandboxID, reqEditors ...RequestEditorFn) (*http.Response, error)

	// ConnectSandboxWithBody performs a POST /sandboxes/{sandboxID}/connect (the `ConnectSandbox` operationId) request,
	// with any type of body and a specified content type.
	ConnectSandboxWithBody(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error)

	// ConnectSandbox performs a POST /sandboxes/{sandboxID}/connect (the `ConnectSandbox` operationId) request.
	// Takes a body of the `application/json` content type.
	ConnectSandbox(ctx context.Context, sandboxID SandboxID, body ConnectSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error)

	// PauseSandbox performs a POST /sandboxes/{sandboxID}/pause (the `PauseSandbox` operationId) request.
	PauseSandbox(ctx context.Context, sandboxID SandboxID, reqEditors ...RequestEditorFn) (*http.Response, error)

	// RefreshSandboxWithBody performs a POST /sandboxes/{sandboxID}/refreshes (the `RefreshSandbox` operationId) request,
	// with any type of body and a specified content type.
	RefreshSandboxWithBody(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error)

	// RefreshSandbox performs a POST /sandboxes/{sandboxID}/refreshes (the `RefreshSandbox` operationId) request.
	// Takes a body of the `application/json` content type.
	RefreshSandbox(ctx context.Context, sandboxID SandboxID, body RefreshSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error)

	// ResumeSandboxWithBody performs a POST /sandboxes/{sandboxID}/resume (the `ResumeSandbox` operationId) request,
	// with any type of body and a specified content type.
	ResumeSandboxWithBody(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error)

	// ResumeSandbox performs a POST /sandboxes/{sandboxID}/resume (the `ResumeSandbox` operationId) request.
	// Takes a body of the `application/json` content type.
	ResumeSandbox(ctx context.Context, sandboxID SandboxID, body ResumeSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error)

	// SetSandboxTimeoutWithBody performs a POST /sandboxes/{sandboxID}/timeout (the `SetSandboxTimeout` operationId) request,
	// with any type of body and a specified content type.
	SetSandboxTimeoutWithBody(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error)

	// SetSandboxTimeout performs a POST /sandboxes/{sandboxID}/timeout (the `SetSandboxTimeout` operationId) request.
	// Takes a body of the `application/json` content type.
	SetSandboxTimeout(ctx context.Context, sandboxID SandboxID, body SetSandboxTimeoutJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error)

	// ListTemplates performs a GET /templates (the `ListTemplates` operationId) request.
	ListTemplates(ctx context.Context, reqEditors ...RequestEditorFn) (*http.Response, error)
}

// ListSandboxes performs a GET /sandboxes (the `ListSandboxes` operationId) request.
func (c *Client) ListSandboxes(ctx context.Context, params *ListSandboxesParams, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewListSandboxesRequest(c.Server, params)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// CreateSandboxWithBody performs a POST /sandboxes (the `CreateSandbox` operationId) request,
// with any type of body and a specified content type.
func (c *Client) CreateSandboxWithBody(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewCreateSandboxRequestWithBody(c.Server, contentType, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// CreateSandbox performs a POST /sandboxes (the `CreateSandbox` operationId) request.
// Takes a body of the `application/json` content type.
func (c *Client) CreateSandbox(ctx context.Context, body CreateSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewCreateSandboxRequest(c.Server, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// DeleteSandbox performs a DELETE /sandboxes/{sandboxID} (the `DeleteSandbox` operationId) request.
func (c *Client) DeleteSandbox(ctx context.Context, sandboxID SandboxID, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewDeleteSandboxRequest(c.Server, sandboxID)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// GetSandbox performs a GET /sandboxes/{sandboxID} (the `GetSandbox` operationId) request.
func (c *Client) GetSandbox(ctx context.Context, sandboxID SandboxID, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewGetSandboxRequest(c.Server, sandboxID)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// ConnectSandboxWithBody performs a POST /sandboxes/{sandboxID}/connect (the `ConnectSandbox` operationId) request,
// with any type of body and a specified content type.
func (c *Client) ConnectSandboxWithBody(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewConnectSandboxRequestWithBody(c.Server, sandboxID, contentType, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// ConnectSandbox performs a POST /sandboxes/{sandboxID}/connect (the `ConnectSandbox` operationId) request.
// Takes a body of the `application/json` content type.
func (c *Client) ConnectSandbox(ctx context.Context, sandboxID SandboxID, body ConnectSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewConnectSandboxRequest(c.Server, sandboxID, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// PauseSandbox performs a POST /sandboxes/{sandboxID}/pause (the `PauseSandbox` operationId) request.
func (c *Client) PauseSandbox(ctx context.Context, sandboxID SandboxID, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewPauseSandboxRequest(c.Server, sandboxID)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// RefreshSandboxWithBody performs a POST /sandboxes/{sandboxID}/refreshes (the `RefreshSandbox` operationId) request,
// with any type of body and a specified content type.
func (c *Client) RefreshSandboxWithBody(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewRefreshSandboxRequestWithBody(c.Server, sandboxID, contentType, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// RefreshSandbox performs a POST /sandboxes/{sandboxID}/refreshes (the `RefreshSandbox` operationId) request.
// Takes a body of the `application/json` content type.
func (c *Client) RefreshSandbox(ctx context.Context, sandboxID SandboxID, body RefreshSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewRefreshSandboxRequest(c.Server, sandboxID, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// ResumeSandboxWithBody performs a POST /sandboxes/{sandboxID}/resume (the `ResumeSandbox` operationId) request,
// with any type of body and a specified content type.
func (c *Client) ResumeSandboxWithBody(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewResumeSandboxRequestWithBody(c.Server, sandboxID, contentType, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// ResumeSandbox performs a POST /sandboxes/{sandboxID}/resume (the `ResumeSandbox` operationId) request.
// Takes a body of the `application/json` content type.
func (c *Client) ResumeSandbox(ctx context.Context, sandboxID SandboxID, body ResumeSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewResumeSandboxRequest(c.Server, sandboxID, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// SetSandboxTimeoutWithBody performs a POST /sandboxes/{sandboxID}/timeout (the `SetSandboxTimeout` operationId) request,
// with any type of body and a specified content type.
func (c *Client) SetSandboxTimeoutWithBody(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewSetSandboxTimeoutRequestWithBody(c.Server, sandboxID, contentType, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// SetSandboxTimeout performs a POST /sandboxes/{sandboxID}/timeout (the `SetSandboxTimeout` operationId) request.
// Takes a body of the `application/json` content type.
func (c *Client) SetSandboxTimeout(ctx context.Context, sandboxID SandboxID, body SetSandboxTimeoutJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewSetSandboxTimeoutRequest(c.Server, sandboxID, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// ListTemplates performs a GET /templates (the `ListTemplates` operationId) request.
func (c *Client) ListTemplates(ctx context.Context, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewListTemplatesRequest(c.Server)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// NewListSandboxesRequest constructs an http.Request for the ListSandboxes method
func NewListSandboxesRequest(server string, params *ListSandboxesParams) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/sandboxes")
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	if params != nil {
		// queryValues collects non-styled parameters (passthrough, JSON)
		// that are safe to round-trip through url.Values.Encode().
		queryValues := queryURL.Query()
		// rawQueryFragments collects pre-encoded query fragments from
		// styled parameters, preserving literal commas as delimiters
		// per the OpenAPI spec (e.g. "color=blue,black,brown").
		var rawQueryFragments []string

		for _, qp := range []struct {
			name    string
			value   any
			typ     string
			explode bool
		}{
			{"metadata", params.Metadata, "string", true},
			{"state", params.State, "array", false},
			{"template", params.Template, "string", true},
			{"nextToken", params.NextToken, "string", true},
			{"limit", params.Limit, "integer", true},
		} {
			// typed nil pointer 装进 interface 后 != nil，需按 Kind 判空。
			if v := reflect.ValueOf(qp.value); v.Kind() == reflect.Pointer && v.IsNil() {
				continue
			}
			queryFrag, err := runtime.StyleParamWithOptions("form", qp.explode, qp.name, qp.value, runtime.StyleParamOptions{ParamLocation: runtime.ParamLocationQuery, Type: qp.typ, Format: ""})
			if err != nil {
				return nil, err
			}
			rawQueryFragments = append(rawQueryFragments, strings.Split(queryFrag, "&")...)
		}

		if encoded := queryValues.Encode(); encoded != "" {
			rawQueryFragments = append(rawQueryFragments, encoded)
		}
		queryURL.RawQuery = strings.Join(rawQueryFragments, "&")
	}

	req, err := http.NewRequest(http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// NewCreateSandboxRequest calls the generic CreateSandbox builder with application/json body
func NewCreateSandboxRequest(server string, body CreateSandboxJSONRequestBody) (*http.Request, error) {
	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)
	return NewCreateSandboxRequestWithBody(server, "application/json", bodyReader)
}

// NewCreateSandboxRequestWithBody constructs an http.Request for the CreateSandbox method, with any body, and a specified content type
func NewCreateSandboxRequestWithBody(server string, contentType string, body io.Reader) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/sandboxes")
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, queryURL.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", contentType)

	return req, nil
}

// NewDeleteSandboxRequest constructs an http.Request for the DeleteSandbox method
func NewDeleteSandboxRequest(server string, sandboxID SandboxID) (*http.Request, error) {
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithOptions("simple", false, "sandboxID", sandboxID, runtime.StyleParamOptions{ParamLocation: runtime.ParamLocationPath, Type: "string", Format: ""})
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/sandboxes/%s", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodDelete, queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// NewGetSandboxRequest constructs an http.Request for the GetSandbox method
func NewGetSandboxRequest(server string, sandboxID SandboxID) (*http.Request, error) {
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithOptions("simple", false, "sandboxID", sandboxID, runtime.StyleParamOptions{ParamLocation: runtime.ParamLocationPath, Type: "string", Format: ""})
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/sandboxes/%s", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// NewConnectSandboxRequest calls the generic ConnectSandbox builder with application/json body
func NewConnectSandboxRequest(server string, sandboxID SandboxID, body ConnectSandboxJSONRequestBody) (*http.Request, error) {
	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)
	return NewConnectSandboxRequestWithBody(server, sandboxID, "application/json", bodyReader)
}

// NewConnectSandboxRequestWithBody constructs an http.Request for the ConnectSandbox method, with any body, and a specified content type
func NewConnectSandboxRequestWithBody(server string, sandboxID SandboxID, contentType string, body io.Reader) (*http.Request, error) {
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithOptions("simple", false, "sandboxID", sandboxID, runtime.StyleParamOptions{ParamLocation: runtime.ParamLocationPath, Type: "string", Format: ""})
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/sandboxes/%s/connect", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, queryURL.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", contentType)

	return req, nil
}

// NewPauseSandboxRequest constructs an http.Request for the PauseSandbox method
func NewPauseSandboxRequest(server string, sandboxID SandboxID) (*http.Request, error) {
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithOptions("simple", false, "sandboxID", sandboxID, runtime.StyleParamOptions{ParamLocation: runtime.ParamLocationPath, Type: "string", Format: ""})
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/sandboxes/%s/pause", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// NewRefreshSandboxRequest calls the generic RefreshSandbox builder with application/json body
func NewRefreshSandboxRequest(server string, sandboxID SandboxID, body RefreshSandboxJSONRequestBody) (*http.Request, error) {
	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)
	return NewRefreshSandboxRequestWithBody(server, sandboxID, "application/json", bodyReader)
}

// NewRefreshSandboxRequestWithBody constructs an http.Request for the RefreshSandbox method, with any body, and a specified content type
func NewRefreshSandboxRequestWithBody(server string, sandboxID SandboxID, contentType string, body io.Reader) (*http.Request, error) {
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithOptions("simple", false, "sandboxID", sandboxID, runtime.StyleParamOptions{ParamLocation: runtime.ParamLocationPath, Type: "string", Format: ""})
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/sandboxes/%s/refreshes", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, queryURL.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", contentType)

	return req, nil
}

// NewResumeSandboxRequest calls the generic ResumeSandbox builder with application/json body
func NewResumeSandboxRequest(server string, sandboxID SandboxID, body ResumeSandboxJSONRequestBody) (*http.Request, error) {
	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)
	return NewResumeSandboxRequestWithBody(server, sandboxID, "application/json", bodyReader)
}

// NewResumeSandboxRequestWithBody constructs an http.Request for the ResumeSandbox method, with any body, and a specified content type
func NewResumeSandboxRequestWithBody(server string, sandboxID SandboxID, contentType string, body io.Reader) (*http.Request, error) {
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithOptions("simple", false, "sandboxID", sandboxID, runtime.StyleParamOptions{ParamLocation: runtime.ParamLocationPath, Type: "string", Format: ""})
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/sandboxes/%s/resume", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, queryURL.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", contentType)

	return req, nil
}

// NewSetSandboxTimeoutRequest calls the generic SetSandboxTimeout builder with application/json body
func NewSetSandboxTimeoutRequest(server string, sandboxID SandboxID, body SetSandboxTimeoutJSONRequestBody) (*http.Request, error) {
	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)
	return NewSetSandboxTimeoutRequestWithBody(server, sandboxID, "application/json", bodyReader)
}

// NewSetSandboxTimeoutRequestWithBody constructs an http.Request for the SetSandboxTimeout method, with any body, and a specified content type
func NewSetSandboxTimeoutRequestWithBody(server string, sandboxID SandboxID, contentType string, body io.Reader) (*http.Request, error) {
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithOptions("simple", false, "sandboxID", sandboxID, runtime.StyleParamOptions{ParamLocation: runtime.ParamLocationPath, Type: "string", Format: ""})
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/sandboxes/%s/timeout", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, queryURL.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", contentType)

	return req, nil
}

// NewListTemplatesRequest constructs an http.Request for the ListTemplates method
func NewListTemplatesRequest(server string) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/templates")
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (c *Client) applyEditors(ctx context.Context, req *http.Request, additionalEditors []RequestEditorFn) error {
	for _, r := range c.RequestEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}
	for _, r := range additionalEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

// ClientWithResponses builds on ClientInterface to offer response payloads
type ClientWithResponses struct {
	ClientInterface
}

// NewClientWithResponses creates a new ClientWithResponses, which wraps
// Client with return type handling
func NewClientWithResponses(server string, opts ...ClientOption) (*ClientWithResponses, error) {
	client, err := NewClient(server, opts...)
	if err != nil {
		return nil, err
	}
	return &ClientWithResponses{client}, nil
}

// WithBaseURL overrides the baseURL.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) error {
		newBaseURL, err := url.Parse(baseURL)
		if err != nil {
			return err
		}
		c.Server = newBaseURL.String()
		return nil
	}
}

// ClientWithResponsesInterface is the interface specification for the client with responses above.
type ClientWithResponsesInterface interface {

	// ListSandboxesWithResponse performs a GET /sandboxes (the `ListSandboxes` operationId) request.
	//
	// Returns a wrapper object for the known response body format(s).
	ListSandboxesWithResponse(ctx context.Context, params *ListSandboxesParams, reqEditors ...RequestEditorFn) (*ListSandboxesResponse, error)

	// CreateSandboxWithBodyWithResponse performs a POST /sandboxes (the `CreateSandbox` operationId) request,
	// with any type of body and a specified content type.
	//
	// Returns a wrapper object for the known response body format(s).
	CreateSandboxWithBodyWithResponse(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*CreateSandboxResponse, error)

	// CreateSandboxWithResponse performs a POST /sandboxes (the `CreateSandbox` operationId) request.
	// Takes a body of the `application/json` content type, and returns a wrapper object for the known response body format(s).
	CreateSandboxWithResponse(ctx context.Context, body CreateSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*CreateSandboxResponse, error)

	// DeleteSandboxWithResponse performs a DELETE /sandboxes/{sandboxID} (the `DeleteSandbox` operationId) request.
	//
	// Returns a wrapper object for the known response body format(s).
	DeleteSandboxWithResponse(ctx context.Context, sandboxID SandboxID, reqEditors ...RequestEditorFn) (*DeleteSandboxResponse, error)

	// GetSandboxWithResponse performs a GET /sandboxes/{sandboxID} (the `GetSandbox` operationId) request.
	//
	// Returns a wrapper object for the known response body format(s).
	GetSandboxWithResponse(ctx context.Context, sandboxID SandboxID, reqEditors ...RequestEditorFn) (*GetSandboxResponse, error)

	// ConnectSandboxWithBodyWithResponse performs a POST /sandboxes/{sandboxID}/connect (the `ConnectSandbox` operationId) request,
	// with any type of body and a specified content type.
	//
	// Returns a wrapper object for the known response body format(s).
	ConnectSandboxWithBodyWithResponse(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*ConnectSandboxResponse, error)

	// ConnectSandboxWithResponse performs a POST /sandboxes/{sandboxID}/connect (the `ConnectSandbox` operationId) request.
	// Takes a body of the `application/json` content type, and returns a wrapper object for the known response body format(s).
	ConnectSandboxWithResponse(ctx context.Context, sandboxID SandboxID, body ConnectSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*ConnectSandboxResponse, error)

	// PauseSandboxWithResponse performs a POST /sandboxes/{sandboxID}/pause (the `PauseSandbox` operationId) request.
	//
	// Returns a wrapper object for the known response body format(s).
	PauseSandboxWithResponse(ctx context.Context, sandboxID SandboxID, reqEditors ...RequestEditorFn) (*PauseSandboxResponse, error)

	// RefreshSandboxWithBodyWithResponse performs a POST /sandboxes/{sandboxID}/refreshes (the `RefreshSandbox` operationId) request,
	// with any type of body and a specified content type.
	//
	// Returns a wrapper object for the known response body format(s).
	RefreshSandboxWithBodyWithResponse(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*RefreshSandboxResponse, error)

	// RefreshSandboxWithResponse performs a POST /sandboxes/{sandboxID}/refreshes (the `RefreshSandbox` operationId) request.
	// Takes a body of the `application/json` content type, and returns a wrapper object for the known response body format(s).
	RefreshSandboxWithResponse(ctx context.Context, sandboxID SandboxID, body RefreshSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*RefreshSandboxResponse, error)

	// ResumeSandboxWithBodyWithResponse performs a POST /sandboxes/{sandboxID}/resume (the `ResumeSandbox` operationId) request,
	// with any type of body and a specified content type.
	//
	// Returns a wrapper object for the known response body format(s).
	ResumeSandboxWithBodyWithResponse(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*ResumeSandboxResponse, error)

	// ResumeSandboxWithResponse performs a POST /sandboxes/{sandboxID}/resume (the `ResumeSandbox` operationId) request.
	// Takes a body of the `application/json` content type, and returns a wrapper object for the known response body format(s).
	ResumeSandboxWithResponse(ctx context.Context, sandboxID SandboxID, body ResumeSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*ResumeSandboxResponse, error)

	// SetSandboxTimeoutWithBodyWithResponse performs a POST /sandboxes/{sandboxID}/timeout (the `SetSandboxTimeout` operationId) request,
	// with any type of body and a specified content type.
	//
	// Returns a wrapper object for the known response body format(s).
	SetSandboxTimeoutWithBodyWithResponse(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*SetSandboxTimeoutResponse, error)

	// SetSandboxTimeoutWithResponse performs a POST /sandboxes/{sandboxID}/timeout (the `SetSandboxTimeout` operationId) request.
	// Takes a body of the `application/json` content type, and returns a wrapper object for the known response body format(s).
	SetSandboxTimeoutWithResponse(ctx context.Context, sandboxID SandboxID, body SetSandboxTimeoutJSONRequestBody, reqEditors ...RequestEditorFn) (*SetSandboxTimeoutResponse, error)

	// ListTemplatesWithResponse performs a GET /templates (the `ListTemplates` operationId) request.
	//
	// Returns a wrapper object for the known response body format(s).
	ListTemplatesWithResponse(ctx context.Context, reqEditors ...RequestEditorFn) (*ListTemplatesResponse, error)
}

// ListSandboxesResponse200Headers the declared response headers of an HTTP 200 response for ListSandboxes
type ListSandboxesResponse200Headers struct {
	XNextToken    *string
	XTotalRunning *int
}

type ListSandboxesResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	// JSON200 the response for an HTTP 200 `application/json` response
	JSON200 *[]Sandbox
	// JSON400 the response for an HTTP 400 `application/json` response
	JSON400 *BadRequest
	// JSON401 the response for an HTTP 401 `application/json` response
	JSON401 *Unauthorized
	// Headers200 the parsed response headers for an HTTP 200 response
	Headers200 *ListSandboxesResponse200Headers
}

// GetJSON200 returns the response for an HTTP 200 `application/json` response
func (r ListSandboxesResponse) GetJSON200() *[]Sandbox {
	return r.JSON200
}

// GetJSON400 returns the response for an HTTP 400 `application/json` response
func (r ListSandboxesResponse) GetJSON400() *BadRequest {
	return r.JSON400
}

// GetJSON401 returns the response for an HTTP 401 `application/json` response
func (r ListSandboxesResponse) GetJSON401() *Unauthorized {
	return r.JSON401
}

// GetBody returns the raw response body bytes
func (r ListSandboxesResponse) GetBody() []byte {
	return r.Body
}

// Status returns HTTPResponse.Status
func (r ListSandboxesResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r ListSandboxesResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// ContentType is a convenience method to retrieve the Content-Type value from the HTTP response headers
func (r ListSandboxesResponse) ContentType() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Header.Get("Content-Type")
	}
	return ""
}

type CreateSandboxResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	// JSON201 the response for an HTTP 201 `application/json` response
	JSON201 *Sandbox
	// JSON400 the response for an HTTP 400 `application/json` response
	JSON400 *BadRequest
	// JSON401 the response for an HTTP 401 `application/json` response
	JSON401 *Unauthorized
	// JSON404 the response for an HTTP 404 `application/json` response
	JSON404 *NotFound
	// JSON429 the response for an HTTP 429 `application/json` response
	JSON429 *RateLimited
	// JSON503 the response for an HTTP 503 `application/json` response
	JSON503 *Unavailable
}

// GetJSON201 returns the response for an HTTP 201 `application/json` response
func (r CreateSandboxResponse) GetJSON201() *Sandbox {
	return r.JSON201
}

// GetJSON400 returns the response for an HTTP 400 `application/json` response
func (r CreateSandboxResponse) GetJSON400() *BadRequest {
	return r.JSON400
}

// GetJSON401 returns the response for an HTTP 401 `application/json` response
func (r CreateSandboxResponse) GetJSON401() *Unauthorized {
	return r.JSON401
}

// GetJSON404 returns the response for an HTTP 404 `application/json` response
func (r CreateSandboxResponse) GetJSON404() *NotFound {
	return r.JSON404
}

// GetJSON429 returns the response for an HTTP 429 `application/json` response
func (r CreateSandboxResponse) GetJSON429() *RateLimited {
	return r.JSON429
}

// GetJSON503 returns the response for an HTTP 503 `application/json` response
func (r CreateSandboxResponse) GetJSON503() *Unavailable {
	return r.JSON503
}

// GetBody returns the raw response body bytes
func (r CreateSandboxResponse) GetBody() []byte {
	return r.Body
}

// Status returns HTTPResponse.Status
func (r CreateSandboxResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r CreateSandboxResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// ContentType is a convenience method to retrieve the Content-Type value from the HTTP response headers
func (r CreateSandboxResponse) ContentType() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Header.Get("Content-Type")
	}
	return ""
}

type DeleteSandboxResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	// JSON404 the response for an HTTP 404 `application/json` response
	JSON404 *NotFound
}

// GetJSON404 returns the response for an HTTP 404 `application/json` response
func (r DeleteSandboxResponse) GetJSON404() *NotFound {
	return r.JSON404
}

// GetBody returns the raw response body bytes
func (r DeleteSandboxResponse) GetBody() []byte {
	return r.Body
}

// Status returns HTTPResponse.Status
func (r DeleteSandboxResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r DeleteSandboxResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// ContentType is a convenience method to retrieve the Content-Type value from the HTTP response headers
func (r DeleteSandboxResponse) ContentType() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Header.Get("Content-Type")
	}
	return ""
}

type GetSandboxResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	// JSON200 the response for an HTTP 200 `application/json` response
	JSON200 *Sandbox
	// JSON404 the response for an HTTP 404 `application/json` response
	JSON404 *NotFound
}

// GetJSON200 returns the response for an HTTP 200 `application/json` response
func (r GetSandboxResponse) GetJSON200() *Sandbox {
	return r.JSON200
}

// GetJSON404 returns the response for an HTTP 404 `application/json` response
func (r GetSandboxResponse) GetJSON404() *NotFound {
	return r.JSON404
}

// GetBody returns the raw response body bytes
func (r GetSandboxResponse) GetBody() []byte {
	return r.Body
}

// Status returns HTTPResponse.Status
func (r GetSandboxResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r GetSandboxResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// ContentType is a convenience method to retrieve the Content-Type value from the HTTP response headers
func (r GetSandboxResponse) ContentType() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Header.Get("Content-Type")
	}
	return ""
}

type ConnectSandboxResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	// JSON200 the response for an HTTP 200 `application/json` response
	JSON200 *Sandbox
	// JSON201 the response for an HTTP 201 `application/json` response
	JSON201 *Sandbox
	// JSON404 the response for an HTTP 404 `application/json` response
	JSON404 *NotFound
	// JSON409 the response for an HTTP 409 `application/json` response
	JSON409 *Conflict
}

// GetJSON200 returns the response for an HTTP 200 `application/json` response
func (r ConnectSandboxResponse) GetJSON200() *Sandbox {
	return r.JSON200
}

// GetJSON201 returns the response for an HTTP 201 `application/json` response
func (r ConnectSandboxResponse) GetJSON201() *Sandbox {
	return r.JSON201
}

// GetJSON404 returns the response for an HTTP 404 `application/json` response
func (r ConnectSandboxResponse) GetJSON404() *NotFound {
	return r.JSON404
}

// GetJSON409 returns the response for an HTTP 409 `application/json` response
func (r ConnectSandboxResponse) GetJSON409() *Conflict {
	return r.JSON409
}

// GetBody returns the raw response body bytes
func (r ConnectSandboxResponse) GetBody() []byte {
	return r.Body
}

// Status returns HTTPResponse.Status
func (r ConnectSandboxResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r ConnectSandboxResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// ContentType is a convenience method to retrieve the Content-Type value from the HTTP response headers
func (r ConnectSandboxResponse) ContentType() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Header.Get("Content-Type")
	}
	return ""
}

type PauseSandboxResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	// JSON404 the response for an HTTP 404 `application/json` response
	JSON404 *NotFound
	// JSON409 the response for an HTTP 409 `application/json` response
	JSON409 *Conflict
}

// GetJSON404 returns the response for an HTTP 404 `application/json` response
func (r PauseSandboxResponse) GetJSON404() *NotFound {
	return r.JSON404
}

// GetJSON409 returns the response for an HTTP 409 `application/json` response
func (r PauseSandboxResponse) GetJSON409() *Conflict {
	return r.JSON409
}

// GetBody returns the raw response body bytes
func (r PauseSandboxResponse) GetBody() []byte {
	return r.Body
}

// Status returns HTTPResponse.Status
func (r PauseSandboxResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r PauseSandboxResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// ContentType is a convenience method to retrieve the Content-Type value from the HTTP response headers
func (r PauseSandboxResponse) ContentType() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Header.Get("Content-Type")
	}
	return ""
}

type RefreshSandboxResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	// JSON400 the response for an HTTP 400 `application/json` response
	JSON400 *BadRequest
	// JSON404 the response for an HTTP 404 `application/json` response
	JSON404 *NotFound
}

// GetJSON400 returns the response for an HTTP 400 `application/json` response
func (r RefreshSandboxResponse) GetJSON400() *BadRequest {
	return r.JSON400
}

// GetJSON404 returns the response for an HTTP 404 `application/json` response
func (r RefreshSandboxResponse) GetJSON404() *NotFound {
	return r.JSON404
}

// GetBody returns the raw response body bytes
func (r RefreshSandboxResponse) GetBody() []byte {
	return r.Body
}

// Status returns HTTPResponse.Status
func (r RefreshSandboxResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r RefreshSandboxResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// ContentType is a convenience method to retrieve the Content-Type value from the HTTP response headers
func (r RefreshSandboxResponse) ContentType() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Header.Get("Content-Type")
	}
	return ""
}

type ResumeSandboxResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	// JSON201 the response for an HTTP 201 `application/json` response
	JSON201 *Sandbox
	// JSON404 the response for an HTTP 404 `application/json` response
	JSON404 *NotFound
	// JSON409 the response for an HTTP 409 `application/json` response
	JSON409 *Conflict
}

// GetJSON201 returns the response for an HTTP 201 `application/json` response
func (r ResumeSandboxResponse) GetJSON201() *Sandbox {
	return r.JSON201
}

// GetJSON404 returns the response for an HTTP 404 `application/json` response
func (r ResumeSandboxResponse) GetJSON404() *NotFound {
	return r.JSON404
}

// GetJSON409 returns the response for an HTTP 409 `application/json` response
func (r ResumeSandboxResponse) GetJSON409() *Conflict {
	return r.JSON409
}

// GetBody returns the raw response body bytes
func (r ResumeSandboxResponse) GetBody() []byte {
	return r.Body
}

// Status returns HTTPResponse.Status
func (r ResumeSandboxResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r ResumeSandboxResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// ContentType is a convenience method to retrieve the Content-Type value from the HTTP response headers
func (r ResumeSandboxResponse) ContentType() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Header.Get("Content-Type")
	}
	return ""
}

type SetSandboxTimeoutResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	// JSON400 the response for an HTTP 400 `application/json` response
	JSON400 *BadRequest
	// JSON404 the response for an HTTP 404 `application/json` response
	JSON404 *NotFound
}

// GetJSON400 returns the response for an HTTP 400 `application/json` response
func (r SetSandboxTimeoutResponse) GetJSON400() *BadRequest {
	return r.JSON400
}

// GetJSON404 returns the response for an HTTP 404 `application/json` response
func (r SetSandboxTimeoutResponse) GetJSON404() *NotFound {
	return r.JSON404
}

// GetBody returns the raw response body bytes
func (r SetSandboxTimeoutResponse) GetBody() []byte {
	return r.Body
}

// Status returns HTTPResponse.Status
func (r SetSandboxTimeoutResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r SetSandboxTimeoutResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// ContentType is a convenience method to retrieve the Content-Type value from the HTTP response headers
func (r SetSandboxTimeoutResponse) ContentType() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Header.Get("Content-Type")
	}
	return ""
}

type ListTemplatesResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	// JSON200 the response for an HTTP 200 `application/json` response
	JSON200 *[]Template
	// JSON401 the response for an HTTP 401 `application/json` response
	JSON401 *Unauthorized
	// JSON403 the response for an HTTP 403 `application/json` response
	JSON403 *Forbidden
}

// GetJSON200 returns the response for an HTTP 200 `application/json` response
func (r ListTemplatesResponse) GetJSON200() *[]Template {
	return r.JSON200
}

// GetJSON401 returns the response for an HTTP 401 `application/json` response
func (r ListTemplatesResponse) GetJSON401() *Unauthorized {
	return r.JSON401
}

// GetJSON403 returns the response for an HTTP 403 `application/json` response
func (r ListTemplatesResponse) GetJSON403() *Forbidden {
	return r.JSON403
}

// GetBody returns the raw response body bytes
func (r ListTemplatesResponse) GetBody() []byte {
	return r.Body
}

// Status returns HTTPResponse.Status
func (r ListTemplatesResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r ListTemplatesResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// ContentType is a convenience method to retrieve the Content-Type value from the HTTP response headers
func (r ListTemplatesResponse) ContentType() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Header.Get("Content-Type")
	}
	return ""
}

// ListSandboxesWithResponse performs a GET /sandboxes (the `ListSandboxes` operationId) request.
//
// Returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) ListSandboxesWithResponse(ctx context.Context, params *ListSandboxesParams, reqEditors ...RequestEditorFn) (*ListSandboxesResponse, error) {
	rsp, err := c.ListSandboxes(ctx, params, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseListSandboxesResponse(rsp)
}

// CreateSandboxWithBodyWithResponse performs a POST /sandboxes (the `CreateSandbox` operationId) request,
// with any type of body and a specified content type.
//
// Returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) CreateSandboxWithBodyWithResponse(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*CreateSandboxResponse, error) {
	rsp, err := c.CreateSandboxWithBody(ctx, contentType, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseCreateSandboxResponse(rsp)
}

// CreateSandboxWithResponse performs a POST /sandboxes (the `CreateSandbox` operationId) request.
// Takes a body of the `application/json` content type, and returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) CreateSandboxWithResponse(ctx context.Context, body CreateSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*CreateSandboxResponse, error) {
	rsp, err := c.CreateSandbox(ctx, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseCreateSandboxResponse(rsp)
}

// DeleteSandboxWithResponse performs a DELETE /sandboxes/{sandboxID} (the `DeleteSandbox` operationId) request.
//
// Returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) DeleteSandboxWithResponse(ctx context.Context, sandboxID SandboxID, reqEditors ...RequestEditorFn) (*DeleteSandboxResponse, error) {
	rsp, err := c.DeleteSandbox(ctx, sandboxID, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseDeleteSandboxResponse(rsp)
}

// GetSandboxWithResponse performs a GET /sandboxes/{sandboxID} (the `GetSandbox` operationId) request.
//
// Returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) GetSandboxWithResponse(ctx context.Context, sandboxID SandboxID, reqEditors ...RequestEditorFn) (*GetSandboxResponse, error) {
	rsp, err := c.GetSandbox(ctx, sandboxID, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseGetSandboxResponse(rsp)
}

// ConnectSandboxWithBodyWithResponse performs a POST /sandboxes/{sandboxID}/connect (the `ConnectSandbox` operationId) request,
// with any type of body and a specified content type.
//
// Returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) ConnectSandboxWithBodyWithResponse(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*ConnectSandboxResponse, error) {
	rsp, err := c.ConnectSandboxWithBody(ctx, sandboxID, contentType, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseConnectSandboxResponse(rsp)
}

// ConnectSandboxWithResponse performs a POST /sandboxes/{sandboxID}/connect (the `ConnectSandbox` operationId) request.
// Takes a body of the `application/json` content type, and returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) ConnectSandboxWithResponse(ctx context.Context, sandboxID SandboxID, body ConnectSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*ConnectSandboxResponse, error) {
	rsp, err := c.ConnectSandbox(ctx, sandboxID, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseConnectSandboxResponse(rsp)
}

// PauseSandboxWithResponse performs a POST /sandboxes/{sandboxID}/pause (the `PauseSandbox` operationId) request.
//
// Returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) PauseSandboxWithResponse(ctx context.Context, sandboxID SandboxID, reqEditors ...RequestEditorFn) (*PauseSandboxResponse, error) {
	rsp, err := c.PauseSandbox(ctx, sandboxID, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParsePauseSandboxResponse(rsp)
}

// RefreshSandboxWithBodyWithResponse performs a POST /sandboxes/{sandboxID}/refreshes (the `RefreshSandbox` operationId) request,
// with any type of body and a specified content type.
//
// Returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) RefreshSandboxWithBodyWithResponse(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*RefreshSandboxResponse, error) {
	rsp, err := c.RefreshSandboxWithBody(ctx, sandboxID, contentType, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseRefreshSandboxResponse(rsp)
}

// RefreshSandboxWithResponse performs a POST /sandboxes/{sandboxID}/refreshes (the `RefreshSandbox` operationId) request.
// Takes a body of the `application/json` content type, and returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) RefreshSandboxWithResponse(ctx context.Context, sandboxID SandboxID, body RefreshSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*RefreshSandboxResponse, error) {
	rsp, err := c.RefreshSandbox(ctx, sandboxID, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseRefreshSandboxResponse(rsp)
}

// ResumeSandboxWithBodyWithResponse performs a POST /sandboxes/{sandboxID}/resume (the `ResumeSandbox` operationId) request,
// with any type of body and a specified content type.
//
// Returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) ResumeSandboxWithBodyWithResponse(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*ResumeSandboxResponse, error) {
	rsp, err := c.ResumeSandboxWithBody(ctx, sandboxID, contentType, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseResumeSandboxResponse(rsp)
}

// ResumeSandboxWithResponse performs a POST /sandboxes/{sandboxID}/resume (the `ResumeSandbox` operationId) request.
// Takes a body of the `application/json` content type, and returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) ResumeSandboxWithResponse(ctx context.Context, sandboxID SandboxID, body ResumeSandboxJSONRequestBody, reqEditors ...RequestEditorFn) (*ResumeSandboxResponse, error) {
	rsp, err := c.ResumeSandbox(ctx, sandboxID, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseResumeSandboxResponse(rsp)
}

// SetSandboxTimeoutWithBodyWithResponse performs a POST /sandboxes/{sandboxID}/timeout (the `SetSandboxTimeout` operationId) request,
// with any type of body and a specified content type.
//
// Returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) SetSandboxTimeoutWithBodyWithResponse(ctx context.Context, sandboxID SandboxID, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*SetSandboxTimeoutResponse, error) {
	rsp, err := c.SetSandboxTimeoutWithBody(ctx, sandboxID, contentType, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseSetSandboxTimeoutResponse(rsp)
}

// SetSandboxTimeoutWithResponse performs a POST /sandboxes/{sandboxID}/timeout (the `SetSandboxTimeout` operationId) request.
// Takes a body of the `application/json` content type, and returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) SetSandboxTimeoutWithResponse(ctx context.Context, sandboxID SandboxID, body SetSandboxTimeoutJSONRequestBody, reqEditors ...RequestEditorFn) (*SetSandboxTimeoutResponse, error) {
	rsp, err := c.SetSandboxTimeout(ctx, sandboxID, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseSetSandboxTimeoutResponse(rsp)
}

// ListTemplatesWithResponse performs a GET /templates (the `ListTemplates` operationId) request.
//
// Returns a wrapper object for the known response body format(s).
func (c *ClientWithResponses) ListTemplatesWithResponse(ctx context.Context, reqEditors ...RequestEditorFn) (*ListTemplatesResponse, error) {
	rsp, err := c.ListTemplates(ctx, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseListTemplatesResponse(rsp)
}

// ParseListSandboxesResponse parses an HTTP response from a ListSandboxesWithResponse call
func ParseListSandboxesResponse(rsp *http.Response) (*ListSandboxesResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &ListSandboxesResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 200:
		var dest []Sandbox
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 400:
		var dest BadRequest
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON400 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 401:
		var dest Unauthorized
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON401 = &dest

	}

	switch {
	case rsp.StatusCode == 200:
		var headers ListSandboxesResponse200Headers
		if values := rsp.Header.Values("X-Next-Token"); len(values) > 0 {
			var value string
			if err := runtime.BindStyledParameterWithOptions("simple", "X-Next-Token", values[0], &value, runtime.BindStyledParameterOptions{ParamLocation: runtime.ParamLocationHeader, Explode: false, Required: false, Type: "string", Format: ""}); err != nil {
				return nil, err
			}
			headers.XNextToken = &value
		}
		if values := rsp.Header.Values("X-Total-Running"); len(values) > 0 {
			var value int
			if err := runtime.BindStyledParameterWithOptions("simple", "X-Total-Running", values[0], &value, runtime.BindStyledParameterOptions{ParamLocation: runtime.ParamLocationHeader, Explode: false, Required: false, Type: "integer", Format: ""}); err != nil {
				return nil, err
			}
			headers.XTotalRunning = &value
		}
		response.Headers200 = &headers
	}

	return response, nil
}

// ParseCreateSandboxResponse parses an HTTP response from a CreateSandboxWithResponse call
func ParseCreateSandboxResponse(rsp *http.Response) (*CreateSandboxResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &CreateSandboxResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 201:
		var dest Sandbox
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON201 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 400:
		var dest BadRequest
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON400 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 401:
		var dest Unauthorized
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON401 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 404:
		var dest NotFound
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON404 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 429:
		var dest RateLimited
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON429 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 503:
		var dest Unavailable
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON503 = &dest

	}

	return response, nil
}

// ParseDeleteSandboxResponse parses an HTTP response from a DeleteSandboxWithResponse call
func ParseDeleteSandboxResponse(rsp *http.Response) (*DeleteSandboxResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &DeleteSandboxResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case rsp.StatusCode == 204:
		break // No content-type

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 404:
		var dest NotFound
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON404 = &dest

	}

	return response, nil
}

// ParseGetSandboxResponse parses an HTTP response from a GetSandboxWithResponse call
func ParseGetSandboxResponse(rsp *http.Response) (*GetSandboxResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &GetSandboxResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 200:
		var dest Sandbox
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 404:
		var dest NotFound
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON404 = &dest

	}

	return response, nil
}

// ParseConnectSandboxResponse parses an HTTP response from a ConnectSandboxWithResponse call
func ParseConnectSandboxResponse(rsp *http.Response) (*ConnectSandboxResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &ConnectSandboxResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 200:
		var dest Sandbox
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 201:
		var dest Sandbox
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON201 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 404:
		var dest NotFound
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON404 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 409:
		var dest Conflict
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON409 = &dest

	}

	return response, nil
}

// ParsePauseSandboxResponse parses an HTTP response from a PauseSandboxWithResponse call
func ParsePauseSandboxResponse(rsp *http.Response) (*PauseSandboxResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &PauseSandboxResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case rsp.StatusCode == 204:
		break // No content-type

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 404:
		var dest NotFound
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON404 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 409:
		var dest Conflict
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON409 = &dest

	}

	return response, nil
}

// ParseRefreshSandboxResponse parses an HTTP response from a RefreshSandboxWithResponse call
func ParseRefreshSandboxResponse(rsp *http.Response) (*RefreshSandboxResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &RefreshSandboxResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case rsp.StatusCode == 204:
		break // No content-type

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 400:
		var dest BadRequest
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON400 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 404:
		var dest NotFound
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON404 = &dest

	}

	return response, nil
}

// ParseResumeSandboxResponse parses an HTTP response from a ResumeSandboxWithResponse call
func ParseResumeSandboxResponse(rsp *http.Response) (*ResumeSandboxResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &ResumeSandboxResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 201:
		var dest Sandbox
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON201 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 404:
		var dest NotFound
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON404 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 409:
		var dest Conflict
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON409 = &dest

	}

	return response, nil
}

// ParseSetSandboxTimeoutResponse parses an HTTP response from a SetSandboxTimeoutWithResponse call
func ParseSetSandboxTimeoutResponse(rsp *http.Response) (*SetSandboxTimeoutResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &SetSandboxTimeoutResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case rsp.StatusCode == 204:
		break // No content-type

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 400:
		var dest BadRequest
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON400 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 404:
		var dest NotFound
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON404 = &dest

	}

	return response, nil
}

// ParseListTemplatesResponse parses an HTTP response from a ListTemplatesWithResponse call
func ParseListTemplatesResponse(rsp *http.Response) (*ListTemplatesResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &ListTemplatesResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 200:
		var dest []Template
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 401:
		var dest Unauthorized
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON401 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 403:
		var dest Forbidden
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON403 = &dest

	}

	return response, nil
}
