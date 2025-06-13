package sse

import "sync"

type Subscriber chan string

type SSEManager struct {
	subscribers map[string][]Subscriber
	mu          sync.RWMutex
}

func NewSSEManager() *SSEManager {
	return &SSEManager{
		subscribers: make(map[string][]Subscriber),
	}
}

func (m *SSEManager) Subscribe(id string) Subscriber {
	ch := make(Subscriber, 10)
	m.mu.Lock()
	m.subscribers[id] = append(m.subscribers[id], ch)
	m.mu.Unlock()
	return ch
}

func (m *SSEManager) Unsubscribe(id string, sub Subscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subs := m.subscribers[id]
	newSubs := make([]Subscriber, 0, len(subs))

	for _, s := range subs {
		if s != sub {
			newSubs = append(newSubs, s)
		}
	}

	if len(newSubs) > 0 {
		m.subscribers[id] = newSubs
	} else {
		delete(m.subscribers, id)
	}

	select {
	case <-sub:
	default:
		close(sub)
	}
}

func (m *SSEManager) Publish(id, message string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, sub := range m.subscribers[id] {
		select {
		case sub <- message:
		default:
		}
	}
}
