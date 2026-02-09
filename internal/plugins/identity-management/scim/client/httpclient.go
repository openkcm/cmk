package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrUnexpectedStatusCode = errors.New("unexpected status code")
)

// DecodeResponse decodes the HTTP response body into the provided type T.
func DecodeResponse[T any](
	ctx context.Context,
	apiName string,
	resp *http.Response,
	expectedStatus int,
) (*T, error) {
	var (
		respErr error
		result  T
	)

	if resp.StatusCode == expectedStatus {
		respErr = json.NewDecoder(resp.Body).Decode(&result)
	} else {
		respErr = fmt.Errorf("%w %s", ErrUnexpectedStatusCode, resp.Status)
	}

	if respErr != nil {
		return nil, fmt.Errorf("invalid response from %s: %w", apiName, respErr)
	}

	return &result, nil
}
