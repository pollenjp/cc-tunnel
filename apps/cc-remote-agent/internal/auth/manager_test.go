package auth

import (
	"context"
	"testing"
	"time"
)

func TestSubscribe_ReceivesData(t *testing.T) {
	m := NewAuthManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := m.Subscribe(ctx)
	if ch == nil {
		t.Fatal("Subscribe returned nil channel")
	}

	// fan-out: send directly to subscribers
	data := []byte("hello\x1b[31mworld\x1b[0m")
	m.subscribersMu.Lock()
	for sub := range m.subscribers {
		select {
		case sub <- data:
		default:
		}
	}
	m.subscribersMu.Unlock()

	select {
	case got, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		if string(got) != string(data) {
			t.Errorf("got %q, want %q", got, data)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for data")
	}
}

func TestSubscribe_ClosedOnContextCancel(t *testing.T) {
	m := NewAuthManager()
	ctx, cancel := context.WithCancel(context.Background())

	ch := m.Subscribe(ctx)
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed after context cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for channel close")
	}
}

func TestSubscribe_MultipleSubscribers(t *testing.T) {
	m := NewAuthManager()

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	ch1 := m.Subscribe(ctx1)
	ch2 := m.Subscribe(ctx2)

	data := []byte("broadcast")
	m.subscribersMu.Lock()
	for sub := range m.subscribers {
		select {
		case sub <- data:
		default:
		}
	}
	m.subscribersMu.Unlock()

	for i, ch := range []<-chan []byte{ch1, ch2} {
		select {
		case got, ok := <-ch:
			if !ok {
				t.Fatalf("ch%d closed unexpectedly", i+1)
			}
			if string(got) != string(data) {
				t.Errorf("ch%d: got %q, want %q", i+1, got, data)
			}
		case <-time.After(time.Second):
			t.Fatalf("ch%d: timeout", i+1)
		}
	}
}
