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

type ErrRequestFailed struct {
	Code int
}

func (e ErrRequestFailed) Error() string {
	return fmt.Sprintf("Request failed: %s", http.StatusText(e.Code))
}

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
	resp, err := c.request(ctx, "GET", "/tasks", nil)
	if err != nil {
		return nil, err
	}

	var res []*tasks.Task
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *Client) UpdateTask(ctx context.Context, uuid string, r *UpdateTaskRequest) (*tasks.Task, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, err
	}

	resp, err := c.request(ctx, "PATCH", "/tasks/"+uuid, bytes.NewReader(body.Bytes()))

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, ErrRequestFailed{resp.StatusCode}
	}

	var res *tasks.Task
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) GetTask(ctx context.Context, uuid string) (*tasks.Task, error) {
	resp, err := c.request(ctx, "GET", "/tasks/"+uuid, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, ErrRequestFailed{resp.StatusCode}
	}

	var res *tasks.Task
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) CreateStorageTask(ctx context.Context, storageTask *tasks.StorageTask) (*tasks.Task, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(storageTask)
	if err != nil {
		return nil, err
	}

	resp, err := c.request(ctx, "POST", "/tasks/storage", bytes.NewReader(body.Bytes()))

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, ErrRequestFailed{resp.StatusCode}
	}

	var res *tasks.Task
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) CreateRetrievalTask(ctx context.Context, retrievalTask *tasks.RetrievalTask) (*tasks.Task, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(retrievalTask)
	if err != nil {
		return nil, err
	}

	resp, err := c.request(ctx, "POST", "/tasks/retrieval", bytes.NewReader(body.Bytes()))

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, ErrRequestFailed{resp.StatusCode}
	}

	var res *tasks.Task
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) request(ctx context.Context, method string, path string, body io.Reader, headers ...string) (*http.Response, error) {
	if len(headers)%2 != 0 {
		return nil, fmt.Errorf("headers must be tuples: key1, value1, key2, value2")
	}
	req, err := http.NewRequest(method, c.endpoint+path, body)
	req = req.WithContext(ctx)

	for i := 0; i < len(headers); i = i + 2 {
		req.Header.Add(headers[i], headers[i+1])
	}

	if err != nil {
		return nil, err
	}
	return c.client.Do(req)
}
