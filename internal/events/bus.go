// Package events is a small in-process pub/sub bus feeding the SSE endpoint.
package events

import (
	"encoding/json"
	"sync"
)

// Message is one server-sent event: a name and a JSON payload.
type Message struct {
	Event string
	Data  []byte
}

// Bus fans messages out to all subscribers. Slow subscribers drop messages
// rather than blocking the publisher; SSE clients recover on the next
// full-state event or by refetching.
type Bus struct {
	mu   sync.Mutex
	subs map[int]chan Message
	next int
}

// NewBus creates an empty bus.
func NewBus() *Bus {
	return &Bus{subs: map[int]chan Message{}}
}

// Subscribe returns a receive channel and a cancel function.
func (b *Bus) Subscribe() (<-chan Message, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	ch := make(chan Message, 64)
	b.subs[id] = ch
	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if sub, ok := b.subs[id]; ok {
			delete(b.subs, id)
			close(sub)
		}
	}
	return ch, cancel
}

// Publish marshals payload and delivers it to every subscriber.
func (b *Bus) Publish(event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	msg := Message{Event: event, Data: data}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- msg:
		default: // drop for slow consumers
		}
	}
}

// SubscriberCount returns the number of connected subscribers.
func (b *Bus) SubscriberCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}
