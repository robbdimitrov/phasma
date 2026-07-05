package pipeline

import (
	"context"
	"testing"
	"time"
)

func TestMonitorReportsStaleRunningPipeline(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	monitor := NewMonitor(time.Minute)
	monitor.now = func() time.Time { return now }

	monitor.Start("outbox-relay")
	now = now.Add(2 * time.Minute)

	statuses := monitor.Snapshot()
	if len(statuses) != 1 || !statuses[0].Stale {
		t.Fatalf("snapshot = %+v, want stale pipeline", statuses)
	}
	if err := monitor.Check(context.Background()); err == nil {
		t.Fatal("Check error = nil, want unhealthy stale pipeline")
	}
}

func TestMonitorProgressClearsStalenessAndCountsWork(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	monitor := NewMonitor(time.Minute)
	monitor.now = func() time.Time { return now }

	monitor.Start("feed-consumer")
	now = now.Add(2 * time.Minute)
	monitor.Progress("feed-consumer", 3, "records")

	statuses := monitor.Snapshot()
	if len(statuses) != 1 {
		t.Fatalf("status count = %d, want 1", len(statuses))
	}
	if statuses[0].Stale {
		t.Fatalf("status = %+v, want not stale after progress", statuses[0])
	}
	if statuses[0].Processed != 3 {
		t.Fatalf("processed = %d, want 3", statuses[0].Processed)
	}
	if err := monitor.Check(context.Background()); err != nil {
		t.Fatalf("Check error = %v, want nil", err)
	}
}

func TestMonitorErrorFailsCheckUntilProgress(t *testing.T) {
	monitor := NewMonitor(time.Minute)
	monitor.Start("outbox-relay")

	monitor.Error("outbox-relay", context.Canceled)
	if err := monitor.Check(context.Background()); err == nil {
		t.Fatal("Check error = nil, want unhealthy after pipeline error")
	}

	monitor.Progress("outbox-relay", 1, "published")
	if err := monitor.Check(context.Background()); err != nil {
		t.Fatalf("Check error after progress = %v, want nil", err)
	}
}

func TestMonitorStoppedPipelineFailsCheck(t *testing.T) {
	monitor := NewMonitor(time.Minute)
	monitor.Start("notifications-consumer")
	monitor.Stop("notifications-consumer")

	if err := monitor.Check(context.Background()); err == nil {
		t.Fatal("Check error = nil, want unhealthy stopped pipeline")
	}
}

func TestMonitorRegisteredPipelineFailsCheckUntilStarted(t *testing.T) {
	monitor := NewMonitor(time.Minute)
	monitor.Register("feed-consumer")

	if err := monitor.Check(context.Background()); err == nil {
		t.Fatal("Check error = nil, want unhealthy registered pipeline")
	}

	monitor.Start("feed-consumer")
	if err := monitor.Check(context.Background()); err != nil {
		t.Fatalf("Check error after start = %v, want nil", err)
	}
}
