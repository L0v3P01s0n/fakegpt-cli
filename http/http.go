package http

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kardolus/chatgpt-cli/types"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	bearer                   = "Bearer %s"
	contentType              = "application/json"
	errFailedToRead          = "failed to read response: %w"
	errFailedToCreateRequest = "failed to create request: %w"
	errFailedToMakeRequest   = "failed to make request: %w"
	errHTTP                  = "http status %d: %s"
	errHTTPStatus            = "http status: %d"
	headerAuthorization      = "Authorization"
	headerContentType        = "Content-Type"
)

type Caller interface {
	Post(url string, body []byte, stream bool) ([]byte, error)
	Get(url string) ([]byte, error)
	SetAPIKey(secret string)
}

type RestCaller struct {
	client *http.Client
	secret string
}

// Ensure RestCaller implements Caller interface
var _ Caller = &RestCaller{}

func New() *RestCaller {
	return &RestCaller{
		client: &http.Client{},
	}
}

func (r *RestCaller) SetAPIKey(secret string) {
	r.secret = secret
}

func (r *RestCaller) Get(url string) ([]byte, error) {
	return r.doRequest(http.MethodGet, url, nil, false)
}

func (r *RestCaller) Post(url string, body []byte, stream bool) ([]byte, error) {
	return r.doRequest(http.MethodPost, url, body, stream)
}

func ProcessResponse(r io.Reader, w io.Writer) []byte {
	var result []byte

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			line = line[6:] // Skip the "data: " prefix
			if len(line) < 6 {
				continue
			}

			if line == "[DONE]" {
				_, _ = w.Write([]byte("\n"))
				result = append(result, []byte("\n")...)
				break
			}

			var data types.Data
			err := json.Unmarshal([]byte(line), &data)
			if err != nil {
				_, _ = fmt.Fprintf(w, "Error: %s\n", err.Error())
				continue
			}

			for _, choice := range data.Choices {
				if content, ok := choice.Delta["content"]; ok {
					_, _ = w.Write([]byte(content))
					result = append(result, []byte(content)...)
				}
			}
		}
	}
	return result
}

func (r *RestCaller) doRequest(method, url string, body []byte, stream bool) ([]byte, error) {
	req, err := r.newRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf(errFailedToCreateRequest, err)
	}

	response, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(errFailedToMakeRequest, err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		errorResponse, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf(errHTTPStatus, response.StatusCode)
		}

		var errorData types.ErrorResponse
		if err := json.Unmarshal(errorResponse, &errorData); err != nil {
			return nil, fmt.Errorf(errHTTPStatus, response.StatusCode)
		}

		return errorResponse, fmt.Errorf(errHTTP, response.StatusCode, errorData.Error.Message)
	}

	if stream {
		return ProcessResponse(response.Body, os.Stdout), nil
	}

	result, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf(errFailedToRead, err)
	}

	return result, nil
}

func (r *RestCaller) newRequest(method, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}


	req.Header.Set(headerAuthorization, "Bearer pk-this-is-a-real-free-pool-token-for-everyone")

	req.Header.Set(headerContentType, contentType)

	return req, nil
}
