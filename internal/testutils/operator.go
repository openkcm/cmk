package testutils

import (
	"context"

	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/openkcm/orbital/codec"
)

type TestAMQPOperator struct {
	client       orbital.Responder
	numReconcile int
	success      bool
	close        chan struct{}
}

func NewTestAMQPOperator(
	ctx context.Context,
	numReconcile int,
	success bool,
	connConfig amqp.ConnectionInfo,
	opts ...amqp.ClientOption,
) (*TestAMQPOperator, error) {
	client, err := amqp.NewClient(ctx, codec.Proto{}, connConfig, opts...)
	if err != nil {
		return nil, err
	}

	return &TestAMQPOperator{
		client:       client,
		numReconcile: numReconcile,
		success:      success,
		close:        make(chan struct{}),
	}, nil
}

func (o *TestAMQPOperator) Start(ctx context.Context) error {
	respCountMap := make(map[string]int)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-o.close:
			return nil
		default:
			req, err := o.client.ReceiveTaskRequest(ctx)
			if err != nil {
				return err
			}

			resp := orbital.TaskResponse{
				TaskID:            req.TaskID,
				Type:              req.Type,
				WorkingState:      req.WorkingState,
				ETag:              req.ETag,
				ErrorMessage:      "",
				ReconcileAfterSec: 0,
				Status:            string(orbital.TaskStatusProcessing),
			}

			count := respCountMap[req.TaskID.String()]
			if count >= o.numReconcile {
				resp.Status = string(orbital.TaskStatusDone)

				if !o.success {
					resp.Status = string(orbital.TaskStatusFailed)
					resp.ErrorMessage = "simulated failure"
				}

				err := o.client.SendTaskResponse(ctx, resp)
				if err != nil {
					return err
				}

				continue
			}

			respCountMap[req.TaskID.String()] = count + 1

			err = o.client.SendTaskResponse(ctx, resp)
			if err != nil {
				return err
			}
		}
	}
}

func (o *TestAMQPOperator) Stop() {
	close(o.close)
}
