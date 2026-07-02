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

const severityError = 1

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

// InitializeResult is the response to the initialize request.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo         `json:"serverInfo"`
}

// ServerCapabilities declares what this server implements.
type ServerCapabilities struct {
	TextDocumentSync       int      `json:"textDocumentSync"` // 1 = full
	CompletionProvider     struct{} `json:"completionProvider"`
	HoverProvider          bool     `json:"hoverProvider"`
	DocumentSymbolProvider bool     `json:"documentSymbolProvider"`
}

// ServerInfo identifies the server to the client.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}
