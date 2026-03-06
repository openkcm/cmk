package testutils

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBinStartup(tb testing.TB, statusServer string, f func() error) {
	tb.Helper()

	errCh := make(chan error, 1)
	go func() {
		errCh <- f()
	}()

	WaitForServer(tb, statusServer)

	select {
	case err := <-errCh:
		assert.NoError(tb, err)
	default:
	}
}

func WaitForServer(tb testing.TB, statusServer string) {
	tb.Helper()

	url := "http://" + statusServer + "/readyz"
	assert.Eventually(tb, func() bool {
		req, err := http.NewRequestWithContext(tb.Context(), http.MethodGet, url, nil)
		if err != nil {
			return false
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 3*time.Second, 100*time.Millisecond)
}
