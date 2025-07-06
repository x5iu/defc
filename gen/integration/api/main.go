//go:build test
// +build test

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

var client Client

func init() {
	log.SetFlags(log.Lshortfile | log.Lmsgprefix)
	log.SetPrefix("[defc] ")
}

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	client = NewClient(&TestOptions{
		client: &http.Client{Transport: &Transport{retryCount: make(map[string]int)}},
	})
	user, err := client.GetUser(ctx, "defc_test_0001")
	if err != nil {
		log.Fatalln(err)
	}
	if user.ID != 1 || user.Name != "defc_test_0001" {
		log.Fatalf("unexpected user: User(id=%d, name=%q)\n", user.ID, user.Name)
	}
	user, err = client.GetUserWithRetry(ctx, "defc_test_002")
	if err != nil {
		log.Fatalln(err)
	}
	if user.ID != 2 || user.Name != "defc_test_002" {
		log.Fatalf("unexpected user with retry=3: User(id=%d, name=%q)\n", user.ID, user.Name)
	}
	resetReader := NewResetReader(`{"name":"defc_test_003"}`)
	user, err = client.CreateUserWithRetry(ctx, resetReader)
	if err != nil {
		log.Fatalln(err)
	}
	if user.ID != 3 || user.Name != "defc_test_003" {
		log.Fatalf("unexpected user with create retry=3: User(id=%d, name=%q)\n", user.ID, user.Name)
	}
}

type Transport struct {
	retryCount map[string]int
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	key := req.URL.Path

	// Increment request count (including the first request)
	t.retryCount[key]++
	count := t.retryCount[key]

	log.Printf("Request to %s (attempt %d)", key, count)

	switch key {
	case "/v1/users/defc_test_0001":
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"code":200,"message":"","data":{"id":1,"name":"defc_test_0001"}}`)),
		}, nil
	case "/v1/users/defc_test_002":
		// Simulate failure for first 3 attempts, success on 4th attempt
		// RETRY=3 means maximum 3 retries, total maximum 4 requests (1 original + 3 retries)
		if count <= 3 {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString(`{"code":500,"message":"Internal Server Error","data":null}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"code":200,"message":"","data":{"id":2,"name":"defc_test_002"}}`)),
		}, nil
	case "/v1/users/":
		// Verify request body is correct
		if req.Body != nil {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				panic(fmt.Sprintf("Failed to read request body: %v", err))
			}
			req.Body.Close()
			expectedBody := `{"name":"defc_test_003"}`
			if string(body) != expectedBody {
				panic(fmt.Sprintf("Request body mismatch: expected %q, got %q", expectedBody, string(body)))
			}
		}
		// Simulate failure for first 3 attempts, success on 4th attempt
		// RETRY=3 means maximum 3 retries, total maximum 4 requests (1 original + 3 retries)
		if count <= 3 {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewBufferString(`{"code":400,"message":"Bad Request","data":null}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(bytes.NewBufferString(`{"code":200,"message":"","data":{"id":3,"name":"defc_test_003"}}`)),
		}, nil
	}
	panic("unreachable")
}

type TestOptions struct {
	client *http.Client
}

func (opts TestOptions) Client() *http.Client {
	return opts.client
}

func (TestOptions) Log(
	_ context.Context,
	method string,
	url string,
	elapse time.Duration,
) {
	fmt.Printf("=== %s %s\nelapse: %s\n", method, url, elapse)
}

//go:generate defc generate -T Client -o client.gen.go --features api/future,api/client,api/log,api/retry
type Client interface {
	Options() *TestOptions
	ResponseHandler() *TestResponseHandler

	// GetUser GET https://localhost:443/v1/users/{{ $.username }}
	GetUser(ctx context.Context, username string) (*User, error)

	// GetUserWithRetry GET RETRY=3 https://localhost:443/v1/users/{{ $.username }}
	GetUserWithRetry(ctx context.Context, username string) (*User, error)

	// CreateUserWithRetry POST RETRY=3 https://localhost:443/v1/users/
	// Content-Type: application/json
	CreateUserWithRetry(ctx context.Context, reader *ResetReader) (*User, error)
}

type User struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type TestResponseHandler struct {
	Code    int
	Message string
	Data    json.RawMessage

	currentMethodName string
}

func (r *TestResponseHandler) FromResponse(name string, resp *http.Response) error {
	defer resp.Body.Close()
	r.currentMethodName = name
	return json.NewDecoder(resp.Body).Decode(r)
}

func (r *TestResponseHandler) ScanValues(values ...any) error {
	if len(values) == 0 {
		return nil
	}
	return json.Unmarshal(r.Data, values[0])
}

func (r *TestResponseHandler) FromBytes(name string, data []byte) error {
	r.currentMethodName = name
	return json.Unmarshal(data, r)
}

func (r *TestResponseHandler) Err() error {
	if r.Code != 200 {
		return fmt.Errorf("(%d) %s", r.Code, r.Message)
	}
	return nil
}

func (r *TestResponseHandler) Break() bool {
	return false
}

type ResetReader struct {
	rd *bytes.Reader
}

func (r *ResetReader) Reset() error {
	if _, err := r.rd.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return nil
}

func (r *ResetReader) Read(p []byte) (n int, err error) {
	return r.rd.Read(p)
}

func NewResetReader(data string) *ResetReader {
	return &ResetReader{rd: bytes.NewReader([]byte(data))}
}
