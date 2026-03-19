// Package scheduler provides task scheduling functionality
package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/agent"
)

// Task represents a scheduled task
type Task struct {
	ID          string
	Type        TaskType
	AgentID     string
	Payload     interface{}
	Status      TaskStatus
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Error       string
}

// TaskType represents the type of task
type TaskType string

const (
	TaskTypeCreateAgent TaskType = "create_agent"
	TaskTypeUpdateAgent TaskType = "update_agent"
	TaskTypeDeleteAgent TaskType = "delete_agent"
	TaskTypeGuide       TaskType = "guide"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// Scheduler manages task execution
type Scheduler struct {
	registry *agent.Registry
	mu       sync.RWMutex
	tasks    map[string]*Task
	queues   map[TaskType]chan *Task
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewScheduler creates a new scheduler
func NewScheduler(registry *agent.Registry) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Scheduler{
		registry: registry,
		tasks:    make(map[string]*Task),
		queues:   make(map[TaskType]chan *Task),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Initialize task queues
	s.queues[TaskTypeCreateAgent] = make(chan *Task, 100)
	s.queues[TaskTypeUpdateAgent] = make(chan *Task, 100)
	s.queues[TaskTypeDeleteAgent] = make(chan *Task, 100)
	s.queues[TaskTypeGuide] = make(chan *Task, 1000)

	return s
}

// Submit adds a new task to the scheduler
func (s *Scheduler) Submit(taskType TaskType, agentID string, payload interface{}) (*Task, error) {
	task := &Task{
		ID:        generateTaskID(),
		Type:      taskType,
		AgentID:   agentID,
		Payload:   payload,
		Status:    TaskStatusPending,
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()

	// Submit to appropriate queue
	queue, exists := s.queues[taskType]
	if !exists {
		return nil, ErrUnknownTaskType
	}

	select {
	case queue <- task:
		log.Printf("Task %s submitted: type=%s, agent=%s", task.ID, taskType, agentID)
		return task, nil
	default:
		return nil, ErrQueueFull
	}
}

// Get retrieves a task by ID
func (s *Scheduler) Get(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[id]
	return task, exists
}

// Start begins processing tasks
func (s *Scheduler) Start() {
	for taskType, queue := range s.queues {
		go s.processQueue(taskType, queue)
	}
	log.Println("Scheduler started")
}

// Stop halts all task processing
func (s *Scheduler) Stop() {
	s.cancel()
	log.Println("Scheduler stopped")
}

func (s *Scheduler) processQueue(taskType TaskType, queue <-chan *Task) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case task := <-queue:
			s.executeTask(task)
		}
	}
}

func (s *Scheduler) executeTask(task *Task) {
	now := time.Now()
	task.Status = TaskStatusRunning
	task.StartedAt = &now

	log.Printf("Executing task %s: type=%s", task.ID, task.Type)

	// TODO: Implement actual task execution logic
	// This would involve calling the appropriate agent service via gRPC

	time.Sleep(100 * time.Millisecond) // Simulated work

	completed := time.Now()
	task.Status = TaskStatusCompleted
	task.CompletedAt = &completed

	log.Printf("Task %s completed", task.ID)
}

func generateTaskID() string {
	return "task_" + time.Now().Format("20060102150405") + "_" + randomString(8)
}

func randomString(n int) string {
	// Simple implementation for now
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}
