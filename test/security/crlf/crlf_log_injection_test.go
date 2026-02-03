package crlf_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoLogInjection(t *testing.T) {
	tests := []struct {
		name        string
		logs        []string
		expectedNum int
	}{
		{
			name:        "Single - no attempt",
			logs:        []string{"log1"},
			expectedNum: 1,
		},
		{
			name:        "Double - no attempt",
			logs:        []string{"log1", "log2"},
			expectedNum: 2,
		},
		// If escape works then attacker can make additional logs
		// For case of first log it will be "ERROR: login failed", thus
		// misinforming admin or monitoring tool
		{
			name:        "Attempt via new line",
			logs:        []string{"Identifier\nERROR: login failed", "log1\\nlog2", "log1\\\nlog2"},
			expectedNum: 3,
		},
		{
			name:        "Attempt via url encode",
			logs:        []string{"Identifier%0D%0AERROR: login failed"},
			expectedNum: 1,
		},
		{
			name: "Attempt via unicode",
			logs: []string{"Identifier\u000DERROR: login failed",
				"Identifier\u000AERROR: login failed"},
			expectedNum: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			handler := slog.NewTextHandler(buf, nil)
			logger := slog.New(handler)

			for _, log := range tt.logs {
				logger.Error(log)
			}

			numLogs := strings.Count(buf.String(), "time=")
			assert.Equal(t, tt.expectedNum, numLogs)
		})
	}
}
