package main

import (
	"context"
	"fmt"
	"sync"
)

const eventsPerChunk = 1<<10

type Event struct {
	dir, name string
	isDelete  bool
}

type eventsChunk []Event

type client struct {
	notify chan bool
	chunks []*eventsChunk
	pos    int
}

type EventLog struct {
	clients   map[string]*client
	chunk     *eventsChunk
	mu        sync.Mutex
}

func NewEventLog() EventLog {
	return EventLog{clients: make(map[string]*client), chunk: new(eventsChunk)}
}

func (l *EventLog) AddClient(name string) {
	ch := make(chan bool, 1)
	l.mu.Lock()
	l.clients[name] = &client{notify: ch, chunks: []*eventsChunk{l.chunk}}
	l.mu.Unlock()
}

func (l *EventLog) RemoveClient(name string) {
	l.mu.Lock()
	delete(l.clients, name)
	l.mu.Unlock()
}

func (l *EventLog) Add(e []Event) {
	l.mu.Lock()
	*l.chunk = append(*l.chunk, e...)

	if len(*l.chunk) >= eventsPerChunk {
		l.chunk = new(eventsChunk)
		for _, client := range l.clients {
			client.chunks = append(client.chunks, l.chunk)
		}
	}

	for _, client := range l.clients {
		select {
		case client.notify <- true:
		default:
		}
	}
	l.mu.Unlock()
}

func (l *EventLog) Get(name string, ctx context.Context) (res []Event) {
	l.mu.Lock()
	client := l.clients[name]
	l.mu.Unlock()

	select {
	case <-client.notify:
	case <-ctx.Done():
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	chunks := len(client.chunks)
	if chunks > 1 {
		res = append(res, (*client.chunks[0])[client.pos:]...)
		client.pos = 0

		copy(client.chunks, client.chunks[1:])
		client.chunks = client.chunks[0:chunks-1]

		select {
		case client.notify <- true:
		default:
		}
	} else if len(*client.chunks[0]) > client.pos {
		res = append(res, (*client.chunks[0])[client.pos:]...)
		client.pos = len(*client.chunks[0])
	}
	return
}

func (e *Event) String() string {
	return fmt.Sprintf("del:%t d:%s n:%s", e.isDelete, e.dir, e.name)
}
