//go:build debugger

package debugadapter

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// Transport is the bidirectional message channel between the DAP client
// and the server. Messages are framed with `Content-Length` headers, the
// same convention used by LSP and DAP.
type Transport struct {
	r       *bufio.Reader
	w       io.Writer
	closer  io.Closer
	writeMu sync.Mutex
}

// NewTransport wraps a reader/writer pair. closer (optional) is called by
// Close to tear down the underlying connection.
func NewTransport(r io.Reader, w io.Writer, closer io.Closer) *Transport {
	return &Transport{r: bufio.NewReader(r), w: w, closer: closer}
}

// ReadMessage reads one DAP message and returns its raw JSON payload.
func (t *Transport) ReadMessage() ([]byte, error) {
	var contentLength int
	for {
		line, err := t.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			n, err := strconv.Atoi(strings.TrimSpace(line[len("Content-Length:"):]))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
			contentLength = n
		}
	}
	if contentLength <= 0 {
		return nil, errors.New("missing or invalid Content-Length header")
	}
	buf := make([]byte, contentLength)
	if _, err := io.ReadFull(t.r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// WriteMessage marshals v as JSON and writes it framed.
func (t *Transport) WriteMessage(v interface{}) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))
	if _, err := t.w.Write([]byte(header)); err != nil {
		return err
	}
	_, err = t.w.Write(payload)
	return err
}

// Close releases any underlying connection.
func (t *Transport) Close() error {
	if t.closer == nil {
		return nil
	}
	return t.closer.Close()
}
