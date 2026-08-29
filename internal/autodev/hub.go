package autodev

import (
	"context"
	"sync"

	"github.com/ymhhh/vibecoding/internal/model"
)

type Hub struct {
	mu        sync.Mutex
	subs      map[string]map[chan model.JobEvent]struct{}
	cancels   map[string]context.CancelFunc
	repoLocks sync.Map // path -> *sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		subs:    make(map[string]map[chan model.JobEvent]struct{}),
		cancels: make(map[string]context.CancelFunc),
	}
}

func (h *Hub) Subscribe(jobID string) (<-chan model.JobEvent, func()) {
	ch := make(chan model.JobEvent, 64)
	h.mu.Lock()
	if h.subs[jobID] == nil {
		h.subs[jobID] = make(map[chan model.JobEvent]struct{})
	}
	h.subs[jobID][ch] = struct{}{}
	h.mu.Unlock()
	unsub := func() {
		h.mu.Lock()
		delete(h.subs[jobID], ch)
		h.mu.Unlock()
		close(ch)
	}
	return ch, unsub
}

func (h *Hub) Publish(jobID string, ev model.JobEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[jobID] {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (h *Hub) Cancel(jobID string) bool {
	h.mu.Lock()
	cancel, ok := h.cancels[jobID]
	h.mu.Unlock()
	if ok && cancel != nil {
		cancel()
		return true
	}
	return false
}

func (h *Hub) lockRepo(path string) func() {
	v, _ := h.repoLocks.LoadOrStore(path, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}
