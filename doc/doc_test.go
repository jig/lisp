package doc

import (
	"embed"
	"encoding/json"
	"testing"
)

//go:embed *.json
var allJSONFiles embed.FS

func TestDocDecode(t *testing.T) {
	files, err := allJSONFiles.ReadDir(".")
	if err != nil {
		t.Fatalf("Failed to read embedded files: %v", err)
	}

	for _, file := range files {
		t.Run(file.Name(), func(t *testing.T) {
			data, err := allJSONFiles.ReadFile(file.Name())
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
		})
	}
}
