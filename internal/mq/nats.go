package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"nuclear-valve-vibration-api/internal/config"
	"nuclear-valve-vibration-api/internal/model"
)

type MessageQueue interface {
	Publish(ctx context.Context, task *model.DiagnosisTask) error
	Subscribe(ctx context.Context, handler func(*model.DiagnosisTask) error) error
	Close() error
}

type natsMQ struct {
	conn       *nats.Conn
	js         nats.JetStreamContext
	cfg        *config.NATSConfig
	sub        *nats.Subscription
	consumerWg sync.WaitGroup
}

func NewNATS(cfg *config.NATSConfig) (MessageQueue, error) {
	var conn *nats.Conn
	var err error

	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		conn, err = nats.Connect(cfg.URL,
			nats.RetryOnFailedConnect(true),
			nats.MaxReconnects(10),
			nats.ReconnectWait(2*time.Second),
			nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
				fmt.Printf("NATS disconnected: %v\n", err)
			}),
			nats.ReconnectHandler(func(nc *nats.Conn) {
				fmt.Printf("NATS reconnected to %s\n", nc.ConnectedUrl())
			}),
		)
		if err == nil {
			break
		}
		fmt.Printf("Failed to connect to NATS (attempt %d/%d): %v\n", i+1, maxRetries, err)
		time.Sleep(time.Second * 2)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS after %d attempts: %w", maxRetries, err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	streamConfig := &nats.StreamConfig{
		Name:     "DIAGNOSIS_TASKS",
		Subjects: []string{cfg.Subject},
		Retention: nats.WorkQueuePolicy,
		MaxMsgs:  -1,
		MaxBytes: -1,
		MaxAge:   24 * time.Hour,
		Storage:  nats.FileStorage,
		Replicas: 1,
	}

	_, err = js.StreamInfo(streamConfig.Name)
	if err != nil {
		if err == nats.ErrStreamNotFound {
			_, err = js.AddStream(streamConfig)
			if err != nil {
				conn.Close()
				return nil, fmt.Errorf("failed to create stream: %w", err)
			}
		} else {
			conn.Close()
			return nil, fmt.Errorf("failed to check stream: %w", err)
		}
	}

	return &natsMQ{
		conn: conn,
		js:   js,
		cfg:  cfg,
	}, nil
}

func (n *natsMQ) Publish(ctx context.Context, task *model.DiagnosisTask) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	msg := &nats.Msg{
		Subject: n.cfg.Subject,
		Data:    data,
		Header:  nats.Header{"Task-ID": []string{task.TaskID}},
	}

	_, err = n.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

func (n *natsMQ) Subscribe(ctx context.Context, handler func(*model.DiagnosisTask) error) error {
	sub, err := n.js.QueueSubscribe(n.cfg.Subject, n.cfg.QueueGroup,
		func(msg *nats.Msg) {
			n.consumerWg.Add(1)
			defer n.consumerWg.Done()

			taskID := msg.Header.Get("Task-ID")
			if taskID == "" {
				_ = msg.Nak()
				return
			}

			var task model.DiagnosisTask
			if err := json.Unmarshal(msg.Data, &task); err != nil {
				_ = msg.Nak()
				return
			}

			if err := handler(&task); err != nil {
				task.RetryCount++
				if task.RetryCount < 3 {
					_ = msg.Nak()
				} else {
					_ = msg.Term()
				}
				return
			}

			_ = msg.Ack()
		},
		nats.ManualAck(),
		nats.AckWait(5*time.Minute),
		nats.MaxDeliver(3),
		nats.Context(ctx),
	)

	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	n.sub = sub
	return nil
}

func (n *natsMQ) Close() error {
	n.consumerWg.Wait()

	if n.sub != nil {
		_ = n.sub.Unsubscribe()
	}

	if n.conn != nil {
		n.conn.Close()
	}

	return nil
}
