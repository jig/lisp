package doc

type Library struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Version     string             `json:"version"`
	Symbols     map[string]Symbols `json:"symbols"`
}

type Symbols struct {
	Description string         `json:"description"`
	Args        []Argument     `json:"args"`
	Returns     Returns        `json:"returns"`
	Errors      []string       `json:"errors"`
	Examples    []Example      `json:"examples"`
	Metadata    map[string]any `json:"metadata"`
}

type Argument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

type Returns struct {
	Description string `json:"description"`
	Type        string `json:"type"`
}

type Example struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}
