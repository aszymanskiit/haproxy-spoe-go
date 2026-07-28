package agent

import (
	"testing"
	"time"

	"github.com/negasus/haproxy-spoe-go/frame"
	"github.com/negasus/haproxy-spoe-go/logger"
	"github.com/negasus/haproxy-spoe-go/request"
)

func TestNew_DefaultMaxFrameSize(t *testing.T) {
	a := New(func(*request.Request) {}, logger.NewNop())
	if a.MaxFrameSize() != frame.DefaultMaxFrameSize {
		t.Fatalf("got %d, want %d", a.MaxFrameSize(), frame.DefaultMaxFrameSize)
	}
}

func TestNewWithOptions_RejectsInvalid(t *testing.T) {
	_, err := NewWithOptions(func(*request.Request) {}, logger.NewNop(), Options{MaxFrameSize: 10})
	if err == nil {
		t.Fatal("expected error for too-small MaxFrameSize")
	}
}

func TestNewWithOptions_ZeroMeansDefault(t *testing.T) {
	a, err := NewWithOptions(func(*request.Request) {}, logger.NewNop(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if a.MaxFrameSize() != frame.DefaultMaxFrameSize {
		t.Fatalf("got %d, want default", a.MaxFrameSize())
	}
}

func TestNewWithOptions_MaxConnectionDuration(t *testing.T) {
	want := 2 * time.Second
	a, err := NewWithOptions(func(*request.Request) {}, logger.NewNop(), Options{
		MaxConnectionDuration: want,
	})
	if err != nil {
		t.Fatal(err)
	}
	if a.MaxConnectionDuration() != want {
		t.Fatalf("got %s, want %s", a.MaxConnectionDuration(), want)
	}
}
