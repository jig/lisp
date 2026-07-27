package log

import (
	"io"

	"github.com/coreos/go-systemd/v22/journal"
)

// SetOutput bypasses destination resolution and writes JSON lines to w.
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	current = &sink{logger: newLogger(w)}
}

// ForceJournal resolves the destination to a fake journal that records
// each send into out.
type JournalEntry struct {
	Message  string
	Priority journal.Priority
	Fields   map[string]string
}

func ForceJournal(out *[]JournalEntry) {
	mu.Lock()
	defer mu.Unlock()
	current = &sink{journald: true}
	journalSend = func(msg string, pri journal.Priority, fields map[string]string) error {
		*out = append(*out, JournalEntry{Message: msg, Priority: pri, Fields: fields})
		return nil
	}
}

// ForceStateFile makes resolution take the XDG state-file path.
func ForceStateFile() {
	mu.Lock()
	defer mu.Unlock()
	current = nil
	journalAvailable = func() bool { return false }
}

// Reset restores resolution state between tests.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	current = nil
	identifier = "lisp"
	runFields = nil
	journalAvailable = journal.Enabled
	journalSend = journal.Send
}
