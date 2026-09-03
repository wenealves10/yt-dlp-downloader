package queues

import "testing"

func TestWorkerConcurrency(t *testing.T) {
	if WorkerConcurrency != 20 {
		t.Fatalf("WorkerConcurrency = %d, want 20", WorkerConcurrency)
	}
}
