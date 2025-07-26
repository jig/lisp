package doc

import (
	_ "embed" // for embedding files
	"encoding/json"
	"fmt"
	"testing"
)

//go:embed core.json
var docCore []byte

func TestDocDecode(t *testing.T) {
	// Test decoding of the core documentation
	var docsLib Library
	err := json.Unmarshal(docCore, &docsLib)
	if err != nil {
		t.Fatalf("Failed to decode core documentation: %v", err)
	}
	fmt.Println("Core documentation decoded successfully with", len(docsLib.Symbols), "entries")
}
