package web

import (
	"sync"
	"time"

	"xkeen-ui/internal/commands"
)

type Event struct {
	Title   string
	Summary string
	Output  string
	At      time.Time
	Success bool
}

type EventStore struct {
	mu   sync.RWMutex
	last *Event
}

func (s *EventStore) Set(event Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.last = &event
}

func (s *EventStore) Get() *Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.last == nil {
		return nil
	}

	cloned := *s.last
	return &cloned
}

func eventFromResult(result commands.Result) Event {
	return Event{
		Title:   result.Title,
		Summary: resultSummary(result),
		Output:  joinOutput(result.Stdout, result.Stderr),
		At:      result.StartedAt,
		Success: result.Success,
	}
}

func eventFromSave(fileName, backupPath string) Event {
	return Event{
		Title:   "Config saved",
		Summary: fileName + " saved successfully",
		Output:  "backup: " + backupPath,
		At:      time.Now().UTC(),
		Success: true,
	}
}
