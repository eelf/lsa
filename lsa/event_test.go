package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestEventLog(t *testing.T) {
	el := NewEventLog()

	resCh := make(chan int)

	go func() {
		el.AddClient("kek")

		streakNoEvents := 0
		eventsProcessed := 0
		for {
			ctx, _ := context.WithTimeout(context.Background(), 10*time.Millisecond)
			evs := el.Get("kek", ctx)
			if len(evs) == 0 {
				streakNoEvents++
				if streakNoEvents > 5 {
					break
				}
				continue
			}
			streakNoEvents = 0

			eventsProcessed += len(evs)
			time.Sleep(20 * time.Millisecond)
		}

		resCh <- eventsProcessed
	}()

	e := Event{"dir", "yeee", true}
	for i := 0; i < 55; i++ {
		e.dir = fmt.Sprint(i)
		el.Add([]Event{e})
		time.Sleep(1 * time.Millisecond)
	}
	if res := <-resCh; res != 55 {
		t.Fatal("events processed", res, "want 55")
	}
}
