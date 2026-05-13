package juju

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRunner struct {
	output []byte
	err    error
	calls  int
}

func (f *fakeRunner) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	f.calls++
	return f.output, f.err
}

func TestPollerSuccess(t *testing.T) {
	runner := &fakeRunner{output: []byte(`{"model":{"name":"prod"},"applications":{}}`)}
	poller := NewPoller(runner, PollConfig{Model: "prod", Timeout: time.Second})
	state, err := poller.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Model != "prod" {
		t.Fatalf("model = %q", state.Model)
	}
	if runner.calls != 1 {
		t.Fatalf("calls = %d", runner.calls)
	}
}

func TestPollerError(t *testing.T) {
	runner := &fakeRunner{err: errors.New("boom")}
	poller := NewPoller(runner, PollConfig{Model: "prod", Timeout: time.Second})
	if _, err := poller.Poll(context.Background()); err == nil {
		t.Fatal("expected poll error")
	}
}
