package gogitutil

import "io"

// NoSign explicitly opts out of go-git's commit.gpgSign auto-signing.
// Returning an empty signature leaves the object without a gpgsig header.
type NoSign struct{}

func (NoSign) Sign(io.Reader) ([]byte, error) {
	return nil, nil
}
