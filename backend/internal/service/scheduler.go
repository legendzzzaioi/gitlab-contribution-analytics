package service

import (
	"log"
	"sync"

	"github.com/robfig/cron/v3"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	cron   *cron.Cron
	mu     sync.Mutex
	entry  cron.EntryID
	syncer *SyncService
}

// StartScheduler 启动定时任务调度器
func StartScheduler(spec string, syncer *SyncService) (*Scheduler, error) {
	c := cron.New()
	entry, err := c.AddFunc(spec, func() {
		result := syncer.StartAsync(false)
		if !result.Started {
			log.Printf("scheduled sync skipped: %s", result.Message)
		}
	})
	if err != nil {
		return nil, err
	}
	c.Start()
	return &Scheduler{cron: c, entry: entry, syncer: syncer}, nil
}

// Stop 停止定时任务调度器
func (s *Scheduler) Stop() {
	if s != nil {
		s.cron.Stop()
	}
}

// Update 更新定时任务
func (s *Scheduler) Update(spec string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cron.Remove(s.entry)
	entry, err := s.cron.AddFunc(spec, func() {
		result := s.syncer.StartAsync(false)
		if !result.Started {
			log.Printf("scheduled sync skipped: %s", result.Message)
		}
	})
	if err != nil {
		return err
	}
	s.entry = entry
	return nil
}
