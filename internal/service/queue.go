package service

import (
	"context"
	"log"
	"sync"
)

type QueueInterface interface {
	EnqueueOrder(userID int, orderNum string)
}

type QueueManager struct {
	service    *AccrualService
	JobQueue   chan OrderJob
	workerPool int
}

var managerInstance QueueManager

type OrderJob struct {
	UserID   int
	OrderNum string
}

func GetQueueManager() *QueueManager {
	var once sync.Once
	once.Do(func() {
		managerInstance = QueueManager{
			service:    GetAccrualService(),
			JobQueue:   make(chan OrderJob, 100),
			workerPool: 5,
		}
		managerInstance.StartWorkers()
	})
	return &managerInstance
}

func (q *QueueManager) StartWorkers() {
	for i := 0; i < q.workerPool; i++ {
		go q.worker(i)
	}
}
func (q *QueueManager) EnqueueOrder(userID int, orderNum string) {
	q.JobQueue <- OrderJob{UserID: userID, OrderNum: orderNum}
}
func (q *QueueManager) worker(id int) {
	for job := range q.JobQueue {
		log.Printf("Worker %d: Processing order %s for user %d", id, job.OrderNum, job.UserID)
		err := q.service.FetchAccrual(context.Background(), job.OrderNum)
		if err != nil {
			log.Printf("Worker %d: Error processing order %s: %v", id, job.OrderNum, err)
		} else {
			log.Printf("Worker %d: Order %s processed successfully", id, job.OrderNum)
		}
	}
}
