package doc

import (
	"encoding/json"
	"testing"
)

func TestDocDecode(t *testing.T) {
	files, err := AllJSONFiles.ReadDir(".")
	if err != nil {
		t.Fatalf("Failed to read embedded files: %v", err)
	}

	totalSymbols := 0

	for _, file := range files {
		t.Run(file.Name(), func(t *testing.T) {
			data, err := AllJSONFiles.ReadFile(file.Name())
			if err != nil {
				t.Errorf("Failed to read file %s: %v", file.Name(), err)
				return
			}

			// Test decoding of the core documentation
			var docsLib Library
			if err := json.Unmarshal(data, &docsLib); err != nil {
				t.Fatalf("Failed to decode core documentation: %v", err)
			}
			t.Logf("Library %q documentation decoded successfully with %d entries", file.Name(), len(docsLib.Symbols))
			totalSymbols += len(docsLib.Symbols)
		})
	}
	t.Logf("Total symbols across all libraries: %d", totalSymbols)
}
