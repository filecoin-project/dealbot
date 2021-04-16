package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/filecoin-project/dealbot/tasks"
	"github.com/urfave/cli/v2"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("controller")

// Client is the API client that performs all operations
// against the dealbot controller.
type Client struct {
	client   *http.Client
	endpoint string
}

// New initializes a new API client
func New(ctx *cli.Context) *Client {
	endpoint := ctx.String("endpoint")

	log.Infow("dealbot controller client initialized", "addr", endpoint)

	return NewFromEndpoint(endpoint)
}

// NewFromEndpoint returns an API client at the given endpoint
func NewFromEndpoint(endpoint string) *Client {
	return &Client{
		client:   &http.Client{},
		endpoint: endpoint,
	}
}

// Close the transport used by the client
func (c *Client) Close() error {
	if t, ok := c.client.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
	return nil
}

func (c *Client) ListTasks(ctx context.Context) ([]*tasks.Task, error) {
	reader, _, err := c.request(ctx, "GET", "/tasks", nil)
	if err != nil {
		return nil, err
	}

	var res []*tasks.Task
	err = json.NewDecoder(reader).Decode(&res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *Client) UpdateTask(ctx context.Context, UUID string, r *UpdateTaskRequest) (io.ReadCloser, int, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, -1, err
	}

	return c.request(ctx, "PATCH", "/tasks/"+UUID, bytes.NewReader(body.Bytes()))
}

func (c *Client) request(ctx context.Context, method string, path string, body io.Reader, headers ...string) (io.ReadCloser, int, error) {
	if len(headers)%2 != 0 {
		return nil, -1, fmt.Errorf("headers must be tuples: key1, value1, key2, value2")
	}
	req, err := http.NewRequest(method, c.endpoint+path, body)
	req = req.WithContext(ctx)

	for i := 0; i < len(headers); i = i + 2 {
		req.Header.Add(headers[i], headers[i+1])
	}

	if err != nil {
		return nil, -1, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, -1, err
	}
	return resp.Body, resp.StatusCode, nil
}
