package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
)

// Task type constants
const (
	TypeEmailDelivery     = "email:delivery"
	TypeWelcomeEmail      = "email:welcome"
	TypePasswordResetEmail = "email:password_reset"
	TypeNotification      = "notification:send"
	TypeReportGeneration  = "report:generate"
	TypeDataCleanup       = "data:cleanup"
)

// EmailDeliveryPayload represents email delivery task payload
type EmailDeliveryPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// WelcomeEmailPayload represents welcome email task payload
type WelcomeEmailPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

// PasswordResetPayload represents password reset email task payload
type PasswordResetPayload struct {
	UserID     string `json:"user_id"`
	Email      string `json:"email"`
	ResetToken string `json:"reset_token"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// NotificationPayload represents notification task payload
type NotificationPayload struct {
	UserID  string                 `json:"user_id"`
	Type    string                 `json:"type"`
	Title   string                 `json:"title"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// ReportPayload represents report generation task payload
type ReportPayload struct {
	ReportID   string    `json:"report_id"`
	ReportType string    `json:"report_type"`
	UserID     string    `json:"user_id"`
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
}

// CleanupPayload represents data cleanup task payload
type CleanupPayload struct {
	Type      string    `json:"type"`
	OlderThan time.Time `json:"older_than"`
}

// NewEmailDeliveryTask creates a new email delivery task
func NewEmailDeliveryTask(to, subject, body string) (*asynq.Task, error) {
	payload, err := json.Marshal(EmailDeliveryPayload{
		To:      to,
		Subject: subject,
		Body:    body,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeEmailDelivery, payload), nil
}

// NewWelcomeEmailTask creates a new welcome email task
func NewWelcomeEmailTask(userID, email, name string) (*asynq.Task, error) {
	payload, err := json.Marshal(WelcomeEmailPayload{
		UserID: userID,
		Email:  email,
		Name:   name,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeWelcomeEmail, payload, asynq.MaxRetry(3)), nil
}

// NewPasswordResetEmailTask creates a new password reset email task
func NewPasswordResetEmailTask(userID, email, resetToken string, expiresAt time.Time) (*asynq.Task, error) {
	payload, err := json.Marshal(PasswordResetPayload{
		UserID:     userID,
		Email:      email,
		ResetToken: resetToken,
		ExpiresAt:  expiresAt,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypePasswordResetEmail, payload, asynq.MaxRetry(3)), nil
}

// NewNotificationTask creates a new notification task
func NewNotificationTask(userID, notificationType, title, message string, data map[string]interface{}) (*asynq.Task, error) {
	payload, err := json.Marshal(NotificationPayload{
		UserID:  userID,
		Type:    notificationType,
		Title:   title,
		Message: message,
		Data:    data,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeNotification, payload, asynq.MaxRetry(5)), nil
}

// NewReportTask creates a new report generation task
func NewReportTask(reportID, reportType, userID string, startDate, endDate time.Time) (*asynq.Task, error) {
	payload, err := json.Marshal(ReportPayload{
		ReportID:   reportID,
		ReportType: reportType,
		UserID:     userID,
		StartDate:  startDate,
		EndDate:    endDate,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeReportGeneration, payload, asynq.MaxRetry(2), asynq.Timeout(30*time.Minute)), nil
}

// NewCleanupTask creates a new data cleanup task
func NewCleanupTask(cleanupType string, olderThan time.Time) (*asynq.Task, error) {
	payload, err := json.Marshal(CleanupPayload{
		Type:      cleanupType,
		OlderThan: olderThan,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDataCleanup, payload, asynq.MaxRetry(1)), nil
}

// ScheduleCleanupTask creates a scheduled cleanup task
func ScheduleCleanupTask(cleanupType string, olderThan time.Time, schedule string) (*asynq.Task, asynq.Option, error) {
	task, err := NewCleanupTask(cleanupType, olderThan)
	if err != nil {
		return nil, nil, err
	}
	// Schedule options would be handled by asynq scheduler
	return task, asynq.Queue("low"), nil
}

// TaskInfo represents information about a task
type TaskInfo struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Queue     string    `json:"queue"`
	Payload   []byte    `json:"payload"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ParsePayload is a helper to parse task payloads
func ParsePayload[T any](task *asynq.Task) (*T, error) {
	var payload T
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return &payload, nil
}

// LogTaskStart logs task start
func LogTaskStart(ctx context.Context, logger *slog.Logger, taskType string) {
	logger.InfoContext(ctx, "starting task",
		slog.String("type", taskType),
	)
}

// LogTaskComplete logs task completion
func LogTaskComplete(ctx context.Context, logger *slog.Logger, taskType string, duration time.Duration) {
	logger.InfoContext(ctx, "task completed",
		slog.String("type", taskType),
		slog.Duration("duration", duration),
	)
}

// LogTaskError logs task error
func LogTaskError(ctx context.Context, logger *slog.Logger, taskType string, err error) {
	logger.ErrorContext(ctx, "task failed",
		slog.String("type", taskType),
		slog.String("error", err.Error()),
	)
}
