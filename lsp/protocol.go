package lsp

import "encoding/json"

// requestMessage is an incoming JSON-RPC 2.0 request or notification
// (notifications carry no ID).
type requestMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

// responseMessage is a successful JSON-RPC 2.0 response. Result is
// always serialized, even when null (required by the spec).
type responseMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  interface{}      `json:"result"`
}

// errorResponseMessage is a failed JSON-RPC 2.0 response.
type errorResponseMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Error   *responseError   `json:"error"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC error codes used by this server.
const (
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// notificationMessage is an outgoing JSON-RPC 2.0 notification.
type notificationMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// --- LSP payload types (minimal subset used by this server) ---

// Position is a zero-based line/character offset in a document.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a [start, end) region of a document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Diagnostic is a single problem reported for a document.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"` // 1=error 2=warning 3=info 4=hint
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

const (
	severityError   = 1
	severityWarning = 2
)

// PublishDiagnosticsParams is the payload of the
// textDocument/publishDiagnostics notification.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// TextDocumentItem is the full document sent on didOpen.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// TextDocumentIdentifier names a document by URI.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// DidOpenTextDocumentParams is the payload of textDocument/didOpen.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// DidChangeTextDocumentParams is the payload of textDocument/didChange.
// The server declares full-document sync, so ContentChanges carries a
// single element with the complete new text.
type DidChangeTextDocumentParams struct {
	TextDocument   TextDocumentIdentifier `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

// DidCloseTextDocumentParams is the payload of textDocument/didClose.
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// TextDocumentPositionParams is shared by completion and hover requests.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// DocumentSymbolParams is the payload of textDocument/documentSymbol.
type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// CompletionItem is a single completion proposal.
type CompletionItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// CompletionItem kinds used by this server.
const (
	completionKindFunction = 3
	completionKindVariable = 6
)

// MarkupContent is a string with a declared format.
type MarkupContent struct {
	Kind  string `json:"kind"` // "plaintext" | "markdown"
	Value string `json:"value"`
}

// Hover is the response to textDocument/hover.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// DocumentSymbol is one entry of the document outline.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// DocumentSymbol kinds used by this server.
const (
	symbolKindFunction = 12
	symbolKindVariable = 13
)

// Location points at a range inside a document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// InitializeParams is the (subset of the) payload of the initialize
// request this server consumes.
type InitializeParams struct {
	InitializationOptions struct {
		// IncludeDirs extends require's module search path, so the
		// editor resolves the same modules the runtime would with -i.
		IncludeDirs []string `json:"includeDirs"`
	} `json:"initializationOptions"`
}

// InitializeResult is the response to the initialize request.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo         `json:"serverInfo"`
}

// ServerCapabilities declares what this server implements.
type ServerCapabilities struct {
	TextDocumentSync           int                   `json:"textDocumentSync"` // 1 = full
	CompletionProvider         struct{}              `json:"completionProvider"`
	HoverProvider              bool                  `json:"hoverProvider"`
	DocumentSymbolProvider     bool                  `json:"documentSymbolProvider"`
	DefinitionProvider         bool                  `json:"definitionProvider"`
	SignatureHelpProvider      SignatureHelpOptions  `json:"signatureHelpProvider"`
	DocumentFormattingProvider bool                  `json:"documentFormattingProvider"`
	RenameProvider             RenameOptions         `json:"renameProvider"`
	ReferencesProvider         bool                  `json:"referencesProvider"`
	SemanticTokensProvider     SemanticTokensOptions `json:"semanticTokensProvider"`
}

// SemanticTokensOptions declares semantic-highlighting support and the
// legend (the ordered token-type / modifier names the encoded token
// integers index into).
type SemanticTokensOptions struct {
	Legend SemanticTokensLegend `json:"legend"`
	Full   bool                 `json:"full"`
}

// SemanticTokensLegend names the token types and modifiers, in the order
// their indices refer to.
type SemanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

// SemanticTokensParams is the payload of textDocument/semanticTokens/full.
type SemanticTokensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// SemanticTokens is the response: a flat array of 5-tuples
// (deltaLine, deltaStartChar, length, tokenType, tokenModifiers),
// each token delta-encoded relative to the previous one.
type SemanticTokens struct {
	Data []int `json:"data"`
}

// ReferenceParams is the payload of textDocument/references.
type ReferenceParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      ReferenceContext       `json:"context"`
}

// ReferenceContext controls whether the symbol's own declaration is
// included in the results.
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// RenameOptions declares rename support, including the prepare step that
// lets the server define the exact range being renamed (so hyphenated
// symbols are selected whole) and reject symbols it cannot safely rename.
type RenameOptions struct {
	PrepareProvider bool `json:"prepareProvider"`
}

// RenameParams is the payload of textDocument/rename.
type RenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	NewName      string                 `json:"newName"`
}

// WorkspaceEdit is the response to textDocument/rename: edits grouped by
// document URI.
type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes"`
}

// DocumentFormattingParams is the payload of textDocument/formatting. The
// FormattingOptions (tab size, insert spaces) are ignored: the formatter
// has a single canonical style.
type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// TextEdit replaces Range with NewText.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// SignatureHelpOptions declares the characters that (re)trigger
// signature help.
type SignatureHelpOptions struct {
	TriggerCharacters   []string `json:"triggerCharacters,omitempty"`
	RetriggerCharacters []string `json:"retriggerCharacters,omitempty"`
}

// SignatureHelp is the response to textDocument/signatureHelp.
type SignatureHelp struct {
	Signatures      []SignatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

// SignatureInformation describes one callable signature.
type SignatureInformation struct {
	Label      string                 `json:"label"`
	Parameters []ParameterInformation `json:"parameters"`
}

// ParameterInformation labels a single parameter (a substring of the
// signature label).
type ParameterInformation struct {
	Label string `json:"label"`
}

// ServerInfo identifies the server to the client.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}
