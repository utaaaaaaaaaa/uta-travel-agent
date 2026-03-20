package scheduler

import "errors"

// Error definitions for scheduler package
var (
	ErrUnknownTaskType = errors.New("unknown task type")
	ErrQueueFull       = errors.New("task queue is full")
	ErrTaskNotFound    = errors.New("task not found")
	ErrSchedulerStopped = errors.New("scheduler has been stopped")
)
