package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/pixperk/goiler/internal/config"
)

// Client represents the Asynq client for enqueueing tasks
type Client struct {
	client *asynq.Client
	logger *slog.Logger
}

// NewClient creates a new worker client
func NewClient(cfg *config.Config, logger *slog.Logger) *Client {
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	return &Client{
		client: asynq.NewClient(redisOpt),
		logger: logger,
	}
}

// Close closes the client connection
func (c *Client) Close() error {
	return c.client.Close()
}

// Enqueue enqueues a task with default options
func (c *Client) Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	info, err := c.client.EnqueueContext(ctx, task, opts...)
	if err != nil {
		c.logger.ErrorContext(ctx, "failed to enqueue task",
			slog.String("type", task.Type()),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	c.logger.InfoContext(ctx, "task enqueued",
		slog.String("type", task.Type()),
		slog.String("id", info.ID),
		slog.String("queue", info.Queue),
	)

	return info, nil
}

// EnqueueIn enqueues a task to be processed after a delay
func (c *Client) EnqueueIn(ctx context.Context, task *asynq.Task, delay time.Duration, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	opts = append(opts, asynq.ProcessIn(delay))
	return c.Enqueue(ctx, task, opts...)
}

// EnqueueAt enqueues a task to be processed at a specific time
func (c *Client) EnqueueAt(ctx context.Context, task *asynq.Task, processAt time.Time, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	opts = append(opts, asynq.ProcessAt(processAt))
	return c.Enqueue(ctx, task, opts...)
}

// EnqueueUnique enqueues a unique task (prevents duplicates)
func (c *Client) EnqueueUnique(ctx context.Context, task *asynq.Task, ttl time.Duration, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	opts = append(opts, asynq.Unique(ttl))
	return c.Enqueue(ctx, task, opts...)
}

// SendEmail enqueues an email delivery task
func (c *Client) SendEmail(ctx context.Context, to, subject, body string) error {
	task, err := NewEmailDeliveryTask(to, subject, body)
	if err != nil {
		return fmt.Errorf("failed to create email task: %w", err)
	}

	_, err = c.Enqueue(ctx, task, asynq.Queue("default"))
	return err
}

// SendWelcomeEmail enqueues a welcome email task
func (c *Client) SendWelcomeEmail(ctx context.Context, userID, email, name string) error {
	task, err := NewWelcomeEmailTask(userID, email, name)
	if err != nil {
		return fmt.Errorf("failed to create welcome email task: %w", err)
	}

	_, err = c.Enqueue(ctx, task, asynq.Queue("default"))
	return err
}

// SendPasswordResetEmail enqueues a password reset email task
func (c *Client) SendPasswordResetEmail(ctx context.Context, userID, email, resetToken string, expiresAt time.Time) error {
	task, err := NewPasswordResetEmailTask(userID, email, resetToken, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to create password reset task: %w", err)
	}

	_, err = c.Enqueue(ctx, task, asynq.Queue("critical"))
	return err
}

// SendNotification enqueues a notification task
func (c *Client) SendNotification(ctx context.Context, userID, notificationType, title, message string, data map[string]interface{}) error {
	task, err := NewNotificationTask(userID, notificationType, title, message, data)
	if err != nil {
		return fmt.Errorf("failed to create notification task: %w", err)
	}

	_, err = c.Enqueue(ctx, task, asynq.Queue("default"))
	return err
}

// GenerateReport enqueues a report generation task
func (c *Client) GenerateReport(ctx context.Context, reportID, reportType, userID string, startDate, endDate time.Time) error {
	task, err := NewReportTask(reportID, reportType, userID, startDate, endDate)
	if err != nil {
		return fmt.Errorf("failed to create report task: %w", err)
	}

	_, err = c.Enqueue(ctx, task, asynq.Queue("low"))
	return err
}

// ScheduleCleanup enqueues a data cleanup task
func (c *Client) ScheduleCleanup(ctx context.Context, cleanupType string, olderThan time.Time) error {
	task, err := NewCleanupTask(cleanupType, olderThan)
	if err != nil {
		return fmt.Errorf("failed to create cleanup task: %w", err)
	}

	_, err = c.Enqueue(ctx, task, asynq.Queue("low"))
	return err
}

// Inspector provides access to inspect queues
type Inspector struct {
	inspector *asynq.Inspector
}

// NewInspector creates a new queue inspector
func NewInspector(cfg *config.Config) *Inspector {
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	return &Inspector{
		inspector: asynq.NewInspector(redisOpt),
	}
}

// GetQueueInfo returns information about a queue
func (i *Inspector) GetQueueInfo(queueName string) (*asynq.QueueInfo, error) {
	return i.inspector.GetQueueInfo(queueName)
}

// ListPendingTasks returns pending tasks in a queue
func (i *Inspector) ListPendingTasks(queueName string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return i.inspector.ListPendingTasks(queueName, opts...)
}

// Close closes the inspector
func (i *Inspector) Close() error {
	return i.inspector.Close()
}
