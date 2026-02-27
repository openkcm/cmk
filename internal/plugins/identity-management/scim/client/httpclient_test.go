package client_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/plugins/identity-management/scim/client"
)

func TestDecodeResponse(t *testing.T) {
	type Response struct {
		Message string `json:"message"`
	}

	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedStatus int
		expectedResult *Response
		expectError    bool
		errorContains  string
	}{
		{
			name:           "Success",
			statusCode:     http.StatusOK,
			responseBody:   `{"message": "success"}`,
			expectedStatus: http.StatusOK,
			expectedResult: &Response{Message: "success"},
			expectError:    false,
		},
		{
			name:           "Unexpected Status Code",
			statusCode:     http.StatusInternalServerError,
			responseBody:   `{"message": "error"}`,
			expectedStatus: http.StatusOK,
			expectedResult: nil,
			expectError:    true,
			errorContains:  "unexpected status code",
		},
		{
			name:           "Invalid JSON",
			statusCode:     http.StatusOK,
			responseBody:   `invalid-json`,
			expectedStatus: http.StatusOK,
			expectedResult: nil,
			expectError:    true,
			errorContains:  "invalid response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, err := w.Write([]byte(tt.responseBody))
				assert.NoError(t, err)
			}))
			defer server.Close()

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
			assert.NoError(t, err)
			resp, err := http.DefaultClient.Do(req)
			assert.NoError(t, err)

			if resp == nil {
				defer resp.Body.Close()
			}

			result, err := client.DecodeResponse[Response](t.Context(), "TestAPI", resp, tt.expectedStatus)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}
