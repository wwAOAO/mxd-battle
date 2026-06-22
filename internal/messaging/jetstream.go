package messaging

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"mxd-battle/internal/config"
)

const battleConsumer = "mxd-battle-worker"

type JetStreamClient struct {
	cfg    config.Config
	logger *slog.Logger
	conn   *nats.Conn
	js     nats.JetStreamContext
	sub    *nats.Subscription
}

func NewJetStreamClient(cfg config.Config, logger *slog.Logger) (*JetStreamClient, error) {
	conn, err := nats.Connect(
		cfg.NATSURL,
		nats.Name(cfg.ServiceName),
		nats.Timeout(5*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Warn("nats disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(conn *nats.Conn) {
			logger.Info("nats reconnected", "url", conn.ConnectedUrl())
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			logger.Info("nats connection closed")
		}),
	)
	if err != nil {
		return nil, err
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &JetStreamClient{
		cfg:    cfg,
		logger: logger,
		conn:   conn,
		js:     js,
	}, nil
}

func (c *JetStreamClient) EnsureBattleStream() error {
	_, err := c.js.StreamInfo(c.cfg.BattleStream)
	if err == nil {
		return nil
	}

	if !errors.Is(err, nats.ErrStreamNotFound) {
		return err
	}

	_, err = c.js.AddStream(&nats.StreamConfig{
		Name:      c.cfg.BattleStream,
		Subjects:  []string{c.cfg.BattleSubject},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		MaxAge:    24 * time.Hour,
	})
	return err
}

func (c *JetStreamClient) SubscribeBattleEvents(ctx context.Context) error {
	sub, err := c.js.PullSubscribe(
		c.cfg.BattleSubject,
		battleConsumer,
		nats.BindStream(c.cfg.BattleStream),
		nats.ManualAck(),
	)
	if err != nil {
		return err
	}

	c.sub = sub

	go c.consume(ctx, sub)
	return nil
}

func (c *JetStreamClient) PublishBattleEvent(subject string, payload []byte) error {
	_, err := c.js.Publish(subject, payload)
	return err
}

func (c *JetStreamClient) consume(ctx context.Context, sub *nats.Subscription) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		messages, err := sub.Fetch(10, nats.MaxWait(time.Second))
		if err != nil {
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			c.logger.Warn("failed to fetch battle events", "error", err)
			continue
		}

		for _, msg := range messages {
			c.handleBattleEvent(msg)
		}
	}
}

func (c *JetStreamClient) handleBattleEvent(msg *nats.Msg) {
	c.logger.Info("received battle event",
		"subject", msg.Subject,
		"bytes", len(msg.Data),
	)

	if err := msg.Ack(); err != nil {
		c.logger.Warn("failed to ack battle event", "error", err)
	}
}

func (c *JetStreamClient) Drain(ctx context.Context) error {
	done := make(chan error, 1)
	go func() {
		if c.sub != nil {
			_ = c.sub.Drain()
		}
		done <- c.conn.Drain()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		c.conn.Close()
		return ctx.Err()
	}
}

func (c *JetStreamClient) Close() {
	c.conn.Close()
}

