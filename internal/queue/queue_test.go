// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package queue

import (
	"context"
	"testing"
	"time"

	"github.com/forke/forke-backend/internal/models"
)

func TestQueueSupersededCancellation(t *testing.T) {
	q := &ReviewQueue{
		jobs:           make(map[string]*models.ReviewJob),
		activeCancels:  make(map[string]context.CancelFunc),
		jobChan:        make(chan *models.ReviewJob, 10),
		maxConcurrency: 2,
	}

	cancelledCount := 0
	dummyCancel := func() {
		cancelledCount++
	}

	// Register dummy in-flight job for repo "owner/sandbox", PR #42, commit SHA A
	prKey := "owner/sandbox#42"
	q.activeCancels[prKey] = dummyCancel

	// Enqueue newer commit SHA B for the same PR
	job, err := q.Enqueue("owner/sandbox", 42, "commit-sha-b", "developer1")
	if err != nil {
		t.Fatalf("unexpected error enqueuing job: %v", err)
	}

	if cancelledCount != 1 {
		t.Errorf("expected superseded job to be cancelled, got count=%d", cancelledCount)
	}

	if job.Status != "queued" {
		t.Errorf("expected job status=queued, got %s", job.Status)
	}
	if job.HeadSHA != "commit-sha-b" {
		t.Errorf("expected job head SHA=commit-sha-b, got %s", job.HeadSHA)
	}

	// Retrieve job by ID
	retrieved := q.GetJob(job.ID)
	if retrieved == nil || retrieved.ID != job.ID {
		t.Errorf("failed to retrieve job by ID")
	}

	// Cancel job
	ok := q.CancelJob(job.ID)
	if !ok {
		t.Errorf("expected CancelJob to return true")
	}
	if retrieved.Status != "cancelled" {
		t.Errorf("expected job status=cancelled, got %s", retrieved.Status)
	}
}

func TestQueueJobProgressLogging(t *testing.T) {
	q := &ReviewQueue{
		jobs:          make(map[string]*models.ReviewJob),
		activeCancels: make(map[string]context.CancelFunc),
		jobChan:       make(chan *models.ReviewJob, 10),
	}

	job, _ := q.Enqueue("owner/sandbox", 10, "sha-1", "user")

	q.mu.Lock()
	job.Progress = 50
	job.Logs = append(job.Logs, "[STAGE] Running tests...")
	q.mu.Unlock()

	res := q.GetJob(job.ID)
	if res.Progress != 50 {
		t.Errorf("expected progress=50, got %d", res.Progress)
	}
	if len(res.Logs) < 2 {
		t.Errorf("expected logs to be appended, got %d lines", len(res.Logs))
	}
	_ = time.Second // keep import
}
