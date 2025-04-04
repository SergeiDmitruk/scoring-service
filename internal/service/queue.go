package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/scoring-service/pkg/logger"
	"go.uber.org/zap"
)

type QueueManager struct {
	mu              sync.Mutex
	service         *AccrualService
	pendingInterval time.Duration
	queue           map[string]struct{}
	workerPool      int
	jobChan         chan string
}

var (
	managerInstance QueueManager
	once            sync.Once
)

func GetQueueManager(service *AccrualService) *QueueManager {
	once.Do(func() {
		managerInstance = QueueManager{
			service:         service,
			pendingInterval: 10 * time.Second,
			queue:           make(map[string]struct{}),
			workerPool:      5,
			jobChan:         make(chan string, 100),
		}
		go managerInstance.Start()
		managerInstance.startWorkers()
	})
	return &managerInstance
}

func (q *QueueManager) Start() {
	ticker := time.NewTicker(q.pendingInterval)
	defer ticker.Stop()

	for range ticker.C {
		q.processPendingOrders()
	}
}

func (q *QueueManager) startWorkers() {
	for i := 0; i < q.workerPool; i++ {
		go q.worker(i)
	}
}

func (q *QueueManager) EnqueueOrder(orderNum string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, exists := q.queue[orderNum]; exists {
		return
	}
	q.queue[orderNum] = struct{}{}
	q.jobChan <- orderNum
}

func (q *QueueManager) DequeueOrder(orderNum string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.queue, orderNum)
}

func (q *QueueManager) worker(id int) {
	for order := range q.jobChan {

		log.Printf("Worker %d: Processing order %s", id, order)
		err := q.service.FetchAccrual(context.Background(), order)

		if err != nil {
			logger.Log.Error("Error processing order", zap.Int("Worker", id), zap.String("order", order), zap.Error(err))
		} else {
			logger.Log.Info("Processed successfully", zap.Int("Worker", id), zap.String("order", order))
		}

		q.DequeueOrder(order)
	}
}

func (q *QueueManager) processPendingOrders() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pendingOrders, err := q.service.db.GetPendingOrders(ctx)
	if err != nil {
		return
	}

	for _, order := range pendingOrders {
		q.EnqueueOrder(order)
	}
}
