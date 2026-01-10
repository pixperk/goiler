package channel

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Event represents a pub/sub event
type Event struct {
	Topic     string
	Payload   interface{}
	Timestamp time.Time
}

// Subscriber represents a subscription to events
type Subscriber struct {
	ID      string
	Topics  []string
	Channel chan Event
	ctx     context.Context
	cancel  context.CancelFunc
}

// PubSub implements an in-process publish/subscribe system
type PubSub struct {
	subscribers map[string]map[string]*Subscriber // topic -> subscriberID -> subscriber
	mu          sync.RWMutex
	logger      *slog.Logger
	bufferSize  int
}

// NewPubSub creates a new PubSub instance
func NewPubSub(logger *slog.Logger, bufferSize int) *PubSub {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &PubSub{
		subscribers: make(map[string]map[string]*Subscriber),
		logger:      logger,
		bufferSize:  bufferSize,
	}
}

// Subscribe creates a new subscription to the specified topics
func (ps *PubSub) Subscribe(ctx context.Context, id string, topics ...string) *Subscriber {
	subCtx, cancel := context.WithCancel(ctx)

	sub := &Subscriber{
		ID:      id,
		Topics:  topics,
		Channel: make(chan Event, ps.bufferSize),
		ctx:     subCtx,
		cancel:  cancel,
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	for _, topic := range topics {
		if ps.subscribers[topic] == nil {
			ps.subscribers[topic] = make(map[string]*Subscriber)
		}
		ps.subscribers[topic][id] = sub
	}

	ps.logger.Info("subscriber added",
		slog.String("id", id),
		slog.Any("topics", topics),
	)

	return sub
}

// Unsubscribe removes a subscriber from all topics
func (ps *PubSub) Unsubscribe(sub *Subscriber) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for _, topic := range sub.Topics {
		if subs, ok := ps.subscribers[topic]; ok {
			delete(subs, sub.ID)
			if len(subs) == 0 {
				delete(ps.subscribers, topic)
			}
		}
	}

	sub.cancel()
	close(sub.Channel)

	ps.logger.Info("subscriber removed", slog.String("id", sub.ID))
}

// Publish publishes an event to all subscribers of the topic
func (ps *PubSub) Publish(topic string, payload interface{}) int {
	event := Event{
		Topic:     topic,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	ps.mu.RLock()
	subs := ps.subscribers[topic]
	ps.mu.RUnlock()

	if len(subs) == 0 {
		return 0
	}

	sent := 0
	for _, sub := range subs {
		select {
		case <-sub.ctx.Done():
			// Subscriber context cancelled, skip
			continue
		case sub.Channel <- event:
			sent++
		default:
			// Channel buffer full, skip to avoid blocking
			ps.logger.Warn("subscriber buffer full, dropping event",
				slog.String("subscriber_id", sub.ID),
				slog.String("topic", topic),
			)
		}
	}

	return sent
}

// PublishAsync publishes an event asynchronously
func (ps *PubSub) PublishAsync(topic string, payload interface{}) {
	go ps.Publish(topic, payload)
}

// GetSubscriberCount returns the number of subscribers for a topic
func (ps *PubSub) GetSubscriberCount(topic string) int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.subscribers[topic])
}

// GetTopics returns all active topics
func (ps *PubSub) GetTopics() []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	topics := make([]string, 0, len(ps.subscribers))
	for topic := range ps.subscribers {
		topics = append(topics, topic)
	}
	return topics
}

// WorkerPool represents a pool of workers processing events
type WorkerPool struct {
	pubsub     *PubSub
	workers    int
	topic      string
	handler    func(Event) error
	subscriber *Subscriber
	wg         sync.WaitGroup
	logger     *slog.Logger
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(pubsub *PubSub, topic string, workers int, handler func(Event) error, logger *slog.Logger) *WorkerPool {
	if workers <= 0 {
		workers = 1
	}
	return &WorkerPool{
		pubsub:  pubsub,
		workers: workers,
		topic:   topic,
		handler: handler,
		logger:  logger,
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start(ctx context.Context) {
	wp.subscriber = wp.pubsub.Subscribe(ctx, "worker-pool-"+wp.topic, wp.topic)

	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i)
	}

	wp.logger.Info("worker pool started",
		slog.String("topic", wp.topic),
		slog.Int("workers", wp.workers),
	)
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() {
	if wp.subscriber != nil {
		wp.pubsub.Unsubscribe(wp.subscriber)
	}
	wp.wg.Wait()
	wp.logger.Info("worker pool stopped", slog.String("topic", wp.topic))
}

// worker processes events from the channel
func (wp *WorkerPool) worker(ctx context.Context, id int) {
	defer wp.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-wp.subscriber.Channel:
			if !ok {
				return
			}

			if err := wp.handler(event); err != nil {
				wp.logger.Error("worker failed to process event",
					slog.Int("worker_id", id),
					slog.String("topic", event.Topic),
					slog.String("error", err.Error()),
				)
			}
		}
	}
}

// Fanout distributes events to multiple channels
type Fanout struct {
	input   chan Event
	outputs []chan Event
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewFanout creates a new fanout
func NewFanout(ctx context.Context, bufferSize int) *Fanout {
	fctx, cancel := context.WithCancel(ctx)
	f := &Fanout{
		input:   make(chan Event, bufferSize),
		outputs: make([]chan Event, 0),
		ctx:     fctx,
		cancel:  cancel,
	}
	go f.run()
	return f
}

// AddOutput adds an output channel
func (f *Fanout) AddOutput(bufferSize int) chan Event {
	ch := make(chan Event, bufferSize)
	f.mu.Lock()
	f.outputs = append(f.outputs, ch)
	f.mu.Unlock()
	return ch
}

// Input returns the input channel
func (f *Fanout) Input() chan<- Event {
	return f.input
}

// Close closes the fanout
func (f *Fanout) Close() {
	f.cancel()
	close(f.input)
}

// run distributes events to all output channels
func (f *Fanout) run() {
	for {
		select {
		case <-f.ctx.Done():
			f.mu.RLock()
			for _, out := range f.outputs {
				close(out)
			}
			f.mu.RUnlock()
			return
		case event, ok := <-f.input:
			if !ok {
				return
			}
			f.mu.RLock()
			for _, out := range f.outputs {
				select {
				case out <- event:
				default:
					// Output buffer full, skip
				}
			}
			f.mu.RUnlock()
		}
	}
}

// Pipeline chains multiple processing stages
type Pipeline struct {
	stages []func(Event) (Event, error)
	input  chan Event
	output chan Event
	errors chan error
	ctx    context.Context
}

// NewPipeline creates a new processing pipeline
func NewPipeline(ctx context.Context, bufferSize int) *Pipeline {
	return &Pipeline{
		stages: make([]func(Event) (Event, error), 0),
		input:  make(chan Event, bufferSize),
		output: make(chan Event, bufferSize),
		errors: make(chan error, bufferSize),
		ctx:    ctx,
	}
}

// AddStage adds a processing stage to the pipeline
func (p *Pipeline) AddStage(stage func(Event) (Event, error)) *Pipeline {
	p.stages = append(p.stages, stage)
	return p
}

// Start starts the pipeline
func (p *Pipeline) Start() {
	go func() {
		for {
			select {
			case <-p.ctx.Done():
				close(p.output)
				close(p.errors)
				return
			case event, ok := <-p.input:
				if !ok {
					close(p.output)
					close(p.errors)
					return
				}

				// Process through all stages
				var err error
				for _, stage := range p.stages {
					event, err = stage(event)
					if err != nil {
						p.errors <- err
						break
					}
				}

				if err == nil {
					p.output <- event
				}
			}
		}
	}()
}

// Input returns the input channel
func (p *Pipeline) Input() chan<- Event {
	return p.input
}

// Output returns the output channel
func (p *Pipeline) Output() <-chan Event {
	return p.output
}

// Errors returns the errors channel
func (p *Pipeline) Errors() <-chan error {
	return p.errors
}
