package doc

type Library struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Version     string             `json:"version"`
	Symbols     map[string]Symbols `json:"symbols"`
}

type Symbols struct {
	Description string     `json:"description"`
	Args        []Argument `json:"args"`
	Returns     Returns    `json:"returns"`
	Errors      []string   `json:"errors"`
	Examples    []Example  `json:"examples"`

	// Metadata can include additional information
	// There are some default attributes:
	// - category
	// - complexity
	// - symbol-type
	Metadata map[string]any `json:"metadata"`
}

type Argument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`

	// TODO(jig): eview the management of this attribute
	Variadic bool `json:"variadic,omitempty"` // Optional, true if the argument can take multiple values
}

type Returns struct {
	Description string `json:"description"`
	Type        string `json:"type"`
}

type Example struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}
