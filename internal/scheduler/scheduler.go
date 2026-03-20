// Package scheduler provides task scheduling functionality
package scheduler

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/agent"
)

// TaskPriority represents task priority level
type TaskPriority int

const (
	PriorityLow    TaskPriority = 0
	PriorityNormal TaskPriority = 1
	PriorityHigh   TaskPriority = 2
)

// ScheduledTask wraps AgentTask with scheduling metadata
type ScheduledTask struct {
	*agent.AgentTask
	Priority    TaskPriority
	RetryCount  int
	MaxRetries  int
	SubmittedAt time.Time
	StartedAt   *time.Time
}

// TaskHandler defines how to handle a task
type TaskHandler func(ctx context.Context, task *ScheduledTask) error

// Scheduler manages task execution with queue, workers, and retry
type Scheduler struct {
	registry *agent.Registry
	mu       sync.RWMutex
	tasks    map[string]*ScheduledTask

	// Task queues by priority
	queues map[TaskPriority]chan *ScheduledTask

	// Worker management
	workerCount int
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc

	// Task handlers
	handlers map[string]TaskHandler

	// Metrics
	completedCount int64
	failedCount    int64
}

// SchedulerConfig for creating a scheduler
type SchedulerConfig struct {
	Registry    *agent.Registry
	WorkerCount int
	QueueSize   int
}

// NewScheduler creates a new scheduler with default config
func NewScheduler(registry *agent.Registry) *Scheduler {
	return NewSchedulerWithConfig(SchedulerConfig{
		Registry:    registry,
		WorkerCount: 4,
		QueueSize:   100,
	})
}

// NewSchedulerWithConfig creates a scheduler with custom config
func NewSchedulerWithConfig(cfg SchedulerConfig) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Scheduler{
		registry:    cfg.Registry,
		tasks:       make(map[string]*ScheduledTask),
		queues:      make(map[TaskPriority]chan *ScheduledTask),
		workerCount: cfg.WorkerCount,
		ctx:         ctx,
		cancel:      cancel,
		handlers:    make(map[string]TaskHandler),
	}

	// Initialize priority queues
	s.queues[PriorityLow] = make(chan *ScheduledTask, cfg.QueueSize)
	s.queues[PriorityNormal] = make(chan *ScheduledTask, cfg.QueueSize)
	s.queues[PriorityHigh] = make(chan *ScheduledTask, cfg.QueueSize)

	// Register default handler
	s.RegisterHandler("create_agent", s.defaultCreateAgentHandler)

	return s
}

// RegisterHandler registers a task handler for a task type
func (s *Scheduler) RegisterHandler(taskType string, handler TaskHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[taskType] = handler
}

// Start begins processing tasks
func (s *Scheduler) Start() {
	// Start workers for each priority level
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}
	log.Printf("Scheduler started with %d workers", s.workerCount)
}

// Stop halts all task processing gracefully
func (s *Scheduler) Stop() {
	log.Println("Scheduler stopping...")
	s.cancel()

	// Close queues
	for _, q := range s.queues {
		close(q)
	}

	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Scheduler stopped gracefully")
	case <-time.After(10 * time.Second):
		log.Println("Scheduler stopped with timeout")
	}
}

// Submit adds a new task to the scheduler
func (s *Scheduler) Submit(task *agent.AgentTask, priority TaskPriority) error {
	return s.SubmitWithRetries(task, priority, 3)
}

// SubmitWithRetries adds a task with custom retry count
func (s *Scheduler) SubmitWithRetries(task *agent.AgentTask, priority TaskPriority, maxRetries int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scheduledTask := &ScheduledTask{
		AgentTask:   task,
		Priority:    priority,
		MaxRetries:  maxRetries,
		RetryCount:  0,
		SubmittedAt: time.Now(),
	}

	s.tasks[task.ID] = scheduledTask

	// Submit to appropriate queue
	queue, exists := s.queues[priority]
	if !exists {
		queue = s.queues[PriorityNormal]
	}

	select {
	case queue <- scheduledTask:
		log.Printf("Task %s submitted with priority %d", task.ID, priority)
		return nil
	default:
		log.Printf("Task queue full, task %s rejected", task.ID)
		return ErrQueueFull
	}
}

// Save saves a task to the scheduler (for compatibility)
func (s *Scheduler) Save(task *agent.AgentTask) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.tasks[task.ID]; ok {
		// Update existing task
		existing.AgentTask = task
	} else {
		// Create new scheduled task
		s.tasks[task.ID] = &ScheduledTask{
			AgentTask:   task,
			Priority:    PriorityNormal,
			MaxRetries:  3,
			SubmittedAt: time.Now(),
		}
	}
}

// Get retrieves a task by ID
func (s *Scheduler) Get(id string) (*agent.AgentTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[id]
	if !exists {
		return nil, false
	}
	return task.AgentTask, true
}

// GetScheduled retrieves a scheduled task by ID
func (s *Scheduler) GetScheduled(id string) (*ScheduledTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[id]
	return task, exists
}

// Delete removes a task from the scheduler
func (s *Scheduler) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tasks, id)
}

// List returns all tasks
func (s *Scheduler) List() []*agent.AgentTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*agent.AgentTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task.AgentTask)
	}
	return tasks
}

// Metrics returns scheduler metrics
func (s *Scheduler) Metrics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"pending_tasks":   len(s.tasks),
		"completed_count": atomic.LoadInt64(&s.completedCount),
		"failed_count":    atomic.LoadInt64(&s.failedCount),
		"worker_count":    s.workerCount,
	}
}

// worker processes tasks from queues
func (s *Scheduler) worker(id int) {
	defer s.wg.Done()

	log.Printf("Worker %d started", id)

	for {
		// Try to get task from highest priority queue first
		var task *ScheduledTask
		var ok bool

		select {
		case <-s.ctx.Done():
			log.Printf("Worker %d stopped", id)
			return
		case task, ok = <-s.queues[PriorityHigh]:
			if !ok {
				return
			}
		case task, ok = <-s.queues[PriorityNormal]:
			if !ok {
				return
			}
		case task, ok = <-s.queues[PriorityLow]:
			if !ok {
				return
			}
		}

		if task != nil {
			s.executeTask(id, task)
		}
	}
}

// executeTask executes a single task with retry logic
func (s *Scheduler) executeTask(workerID int, task *ScheduledTask) {
	ctx := s.ctx

	// Update task status
	now := time.Now()
	task.StartedAt = &now
	task.Status = agent.TaskStatusRunning
	log.Printf("Worker %d: Executing task %s (attempt %d/%d)", workerID, task.ID, task.RetryCount+1, task.MaxRetries+1)

	// Get handler
	s.mu.RLock()
	handler, exists := s.handlers["create_agent"] // Default handler
	s.mu.RUnlock()

	var err error
	if exists {
		err = handler(ctx, task)
	} else {
		err = s.defaultCreateAgentHandler(ctx, task)
	}

	if err != nil {
		log.Printf("Worker %d: Task %s failed: %v", workerID, task.ID, err)

		// Check if we should retry
		if task.RetryCount < task.MaxRetries {
			task.RetryCount++
			task.Status = agent.TaskStatusPending

			// Re-queue with exponential backoff
			go func() {
				backoff := time.Duration(task.RetryCount) * time.Second
				time.Sleep(backoff)

				s.mu.Lock()
				queue := s.queues[task.Priority]
				s.mu.Unlock()

				select {
				case queue <- task:
					log.Printf("Task %s re-queued (attempt %d)", task.ID, task.RetryCount+1)
				default:
					log.Printf("Task %s re-queue failed (queue full)", task.ID)
				}
			}()
		} else {
			// Max retries exceeded
			task.Status = agent.TaskStatusFailed
			task.Error = err.Error()
			completed := time.Now()
			task.CompletedAt = &completed
			atomic.AddInt64(&s.failedCount, 1)
			log.Printf("Worker %d: Task %s failed permanently after %d attempts", workerID, task.ID, task.RetryCount+1)
		}
	} else {
		// Success
		completed := time.Now()
		task.Status = agent.TaskStatusCompleted
		task.CompletedAt = &completed
		task.DurationSeconds = completed.Sub(task.SubmittedAt).Seconds()
		atomic.AddInt64(&s.completedCount, 1)
		log.Printf("Worker %d: Task %s completed in %.2fs", workerID, task.ID, task.DurationSeconds)
	}
}

// defaultCreateAgentHandler handles agent creation tasks
func (s *Scheduler) defaultCreateAgentHandler(ctx context.Context, task *ScheduledTask) error {
	// This is a placeholder - the actual handler will be injected
	// when we integrate with the agent service

	// Simulate work for now
	select {
	case <-time.After(5 * time.Second):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}