package testutils

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/openkcm/orbital/codec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/log"
)

// TestAMQPConfig provides a default AMQP configuration for testing
var TestAMQPConfig = config.AMQP{
	URL:    "amqp://guest:guest@localhost:5672",
	Target: "target",
	Source: "source",
}

type TestAMQPOperator struct {
	t            *testing.T
	client       *amqp.Client
	numReconcile int
	success      bool
	close        chan struct{}
	respCountMap map[string]int
	mu           sync.RWMutex // Add mutex for thread safety
}

func NewTestAMQPOperator(
	t *testing.T,
	numReconcile int,
	success bool,
	connConfig amqp.ConnectionInfo,
	opts ...amqp.ClientOption,
) *TestAMQPOperator {
	t.Helper()

	// Use separate context for the operator to allow shutdown
	// in the t.Cleanup method and cancel it independently
	ctx := context.Background()

	client, err := amqp.NewClient(ctx, codec.Proto{}, connConfig, opts...)
	if err != nil {
		require.NoError(t, err)
	}

	operator := TestAMQPOperator{
		client:       client,
		numReconcile: numReconcile,
		success:      success,
		close:        make(chan struct{}),
		respCountMap: make(map[string]int),
	}

	t.Cleanup(func() {
		operator.Stop(ctx)
		operator.Reset()
	})

	return &operator
}

// Reset clears the internal state for test isolation
func (o *TestAMQPOperator) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.respCountMap = make(map[string]int)
}

func (o *TestAMQPOperator) Start(ctx context.Context) {
	log.Debug(ctx, "starting test AMQP operator")

	for {
		select {
		case <-ctx.Done():
			return
		case <-o.close:
			return
		default:
			req, err := o.client.ReceiveTaskRequest(ctx)
			if err != nil {
				log.Debug(ctx, "operator error receiving task request", log.ErrorAttr(err))
				return
			}

			log.Debug(ctx, "operator received task request", slog.Any("request", req), slog.Any("instance", o))

			o.mu.Lock()
			log.Debug(ctx, "map state before handling", slog.Any("respCountMap", o.respCountMap))

			resp := orbital.TaskResponse{
				TaskID:            req.TaskID,
				Type:              req.Type,
				WorkingState:      req.WorkingState,
				ETag:              req.ETag,
				ErrorMessage:      "",
				ReconcileAfterSec: 0,
				Status:            string(orbital.TaskStatusProcessing),
			}

			count := o.respCountMap[req.TaskID.String()]
			if count >= o.numReconcile {
				resp.Status = string(orbital.TaskStatusDone)

				if !o.success {
					resp.Status = string(orbital.TaskStatusFailed)
					resp.ErrorMessage = "simulated failure"
				}

				o.mu.Unlock() // Unlock before sending response

				err = o.sendTaskResponse(ctx, resp, count)
				if err != nil {
					require.NoError(o.t, err, "error sending final task response")
				}

				continue
			}

			o.respCountMap[req.TaskID.String()] = count + 1
			log.Debug(ctx, "map state after handling", slog.Any("respCountMap", o.respCountMap))
			o.mu.Unlock()

			err = o.sendTaskResponse(ctx, resp, count)
			if err != nil {
				require.NoError(o.t, err, "error sending task response")
			}
		}
	}
}

func (o *TestAMQPOperator) Stop(ctx context.Context) {
	close(o.close)
	err := o.client.Close(ctx)
	assert.NoError(o.t, err)
	log.Debug(ctx, "stopped test AMQP operator")
}

func (o *TestAMQPOperator) sendTaskResponse(ctx context.Context, resp orbital.TaskResponse, count int) error {
	err := o.client.SendTaskResponse(ctx, resp)
	if err != nil {
		log.Debug(ctx, "operator error sending task response", log.ErrorAttr(err))
		return err
	}

	log.Debug(ctx, "operator sent task response", slog.Any("count", count), slog.Any("instance", o),
		slog.Any("response", resp))

	return nil
}
