// Package lsp implements a minimal Language Server Protocol server for
// jig/lisp: live parse diagnostics, completion, hover and document
// outline. It reuses the interpreter's reader (positions and errors are
// exactly what the interpreter itself would report) and an interpreter
// environment for builtin symbols.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/jig/lisp/lib/require"
	"github.com/jig/lisp/types"
)

// Server is the LSP server. One instance handles one client session.
type Server struct {
	t *Transport

	// env supplies builtin symbol names for completion and hover. It is
	// read-only from the server's perspective; nil disables env-backed
	// features (document-local definitions still work).
	env types.EnvType

	mu   sync.Mutex
	docs map[string]*document // uri → open document

	shutdown bool
}

// document is one open text document and its latest analysis.
type document struct {
	content  string
	analysis *analysis
	external []externalDef // definitions imported from require'd modules
}

// externalDef is a definition imported from a require'd module.
type externalDef struct {
	definition
	path string // absolute path of the module file
}

// NewServer constructs a server. env supplies builtin symbols for
// completion and hover; pass the same environment the interpreter would
// run with.
func NewServer(t *Transport, env types.EnvType) *Server {
	return &Server{
		t:    t,
		env:  env,
		docs: map[string]*document{},
	}
}

// Run reads and dispatches client messages until the client sends
// `exit` or the transport closes.
func (s *Server) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		raw, err := s.t.ReadMessage()
		if err != nil {
			return nil // client closed the connection
		}
		var req requestMessage
		if err := json.Unmarshal(raw, &req); err != nil {
			continue
		}
		if req.Method == "exit" {
			return nil
		}
		s.dispatch(&req)
	}
}

func (s *Server) dispatch(req *requestMessage) {
	switch req.Method {
	case "initialize":
		s.respond(req, InitializeResult{
			Capabilities: ServerCapabilities{
				TextDocumentSync:       1, // full document sync
				HoverProvider:          true,
				DocumentSymbolProvider: true,
			},
			ServerInfo: ServerInfo{Name: "jig-lisp-lsp"},
		})
	case "initialized":
		// notification; nothing to do
	case "shutdown":
		s.mu.Lock()
		s.shutdown = true
		s.mu.Unlock()
		s.respond(req, nil)
	case "textDocument/didOpen":
		var p DidOpenTextDocumentParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return
		}
		s.updateDocument(p.TextDocument.URI, p.TextDocument.Text)
	case "textDocument/didChange":
		var p DidChangeTextDocumentParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return
		}
		if len(p.ContentChanges) == 0 {
			return
		}
		// Full sync: the last change carries the complete document.
		s.updateDocument(p.TextDocument.URI, p.ContentChanges[len(p.ContentChanges)-1].Text)
	case "textDocument/didClose":
		var p DidCloseTextDocumentParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return
		}
		s.mu.Lock()
		delete(s.docs, p.TextDocument.URI)
		s.mu.Unlock()
		s.notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{
			URI: p.TextDocument.URI, Diagnostics: []Diagnostic{},
		})
	case "textDocument/completion":
		s.handleCompletion(req)
	case "textDocument/hover":
		s.handleHover(req)
	case "textDocument/documentSymbol":
		s.handleDocumentSymbol(req)
	default:
		if req.ID != nil {
			s.respondError(req, codeMethodNotFound, "unsupported method: "+req.Method)
		}
		// unknown notifications are ignored per the spec
	}
}

// updateDocument stores the new content, re-analyses it and publishes
// the resulting diagnostics.
func (s *Server) updateDocument(uri, content string) {
	anal := analyseDocument(uri, content)
	external, requireDiags := resolveRequires(anal)
	s.mu.Lock()
	s.docs[uri] = &document{content: content, analysis: anal, external: external}
	s.mu.Unlock()
	diags := append([]Diagnostic{}, anal.diagnostics...)
	diags = append(diags, requireDiags...)
	diags = append(diags, s.unknownSymbolDiagnostics(anal, external)...)
	s.notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI: uri, Diagnostics: diags,
	})
}

// resolveRequires statically resolves the document's `(require …)`
// forms with the same search cascade the runtime uses, analyses the
// resolved files (transitively, cycle-safe) and returns their
// definitions under the same names the runtime would publish:
// `module/name` (or `alias/name` with :as), plus the unqualified names
// listed in :refer. A require that cannot be resolved or read yields a
// warning on the `require` symbol itself; nested failures are silent
// (they belong to the module's own diagnostics when opened).
func resolveRequires(anal *analysis) ([]externalDef, []Diagnostic) {
	var out []externalDef
	var diags []Diagnostic
	analysed := map[string]*analysis{} // path → analysis; nil = unreadable
	var walk func(a *analysis, depth int, report bool)
	walk = func(a *analysis, depth int, report bool) {
		if depth > 16 {
			return
		}
		for _, req := range a.requires {
			path, err := require.Resolve(req.module)
			if err != nil {
				if report {
					diags = append(diags, Diagnostic{
						Range:    symbolRange(req.headPos, "require"),
						Severity: severityWarning,
						Source:   "lisp",
						Message:  err.Error(),
					})
				}
				continue
			}
			ma, seen := analysed[path]
			if !seen {
				content, err := os.ReadFile(path)
				if err != nil {
					analysed[path] = nil
					if report {
						diags = append(diags, Diagnostic{
							Range:    symbolRange(req.headPos, "require"),
							Severity: severityWarning,
							Source:   "lisp",
							Message:  fmt.Sprintf("require: %v", err),
						})
					}
					continue
				}
				ma = analyseDocument(path, string(content))
				analysed[path] = ma
				// Each file is analysed and recursed once; publication
				// below happens per require occurrence (aliases differ).
				walk(ma, depth+1, false)
			}
			if ma == nil {
				continue
			}
			prefix := req.module
			if req.alias != "" {
				prefix = req.alias
			}
			referred := map[string]bool{}
			for _, name := range req.refers {
				referred[name] = true
			}
			for _, d := range ma.defs {
				qualified := d
				qualified.name = prefix + "/" + d.name
				out = append(out, externalDef{definition: qualified, path: path})
				if referred[d.name] {
					out = append(out, externalDef{definition: d, path: path})
				}
			}
		}
	}
	walk(anal, 0, true)
	return out, diags
}

// unknownSymbolDiagnostics warns about symbols used in call position
// that resolve nowhere: not bound anywhere in the document, and not
// present in the interpreter environment. It is a warning rather than
// an error because the symbol may be defined at runtime (load-file of
// another script, dynamic def).
func (s *Server) unknownSymbolDiagnostics(anal *analysis, external []externalDef) []Diagnostic {
	if s.env == nil {
		return nil
	}
	imported := map[string]bool{}
	for _, d := range external {
		imported[d.name] = true
	}
	var out []Diagnostic
	reported := map[string]bool{}
	for _, call := range anal.calls {
		if anal.bound[call.name] || imported[call.name] || reported[call.name] {
			continue
		}
		if _, err := s.env.Get(types.Symbol{Val: call.name}); err == nil {
			continue
		}
		reported[call.name] = true
		out = append(out, Diagnostic{
			Range:    symbolRange(call.pos, call.name),
			Severity: severityWarning,
			Source:   "lisp",
			Message:  fmt.Sprintf("unknown symbol '%s'", call.name),
		})
	}
	return out
}

func (s *Server) handleCompletion(req *requestMessage) {
	var p TextDocumentPositionParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.respondError(req, codeInvalidParams, err.Error())
		return
	}
	s.mu.Lock()
	doc := s.docs[p.TextDocument.URI]
	s.mu.Unlock()

	seen := map[string]bool{}
	items := []CompletionItem{}

	// Document-local definitions first: they are the most relevant.
	if doc != nil {
		for _, d := range doc.analysis.defs {
			if seen[d.name] {
				continue
			}
			seen[d.name] = true
			items = append(items, CompletionItem{
				Label:  d.name,
				Kind:   completionKindOf(d.kind),
				Detail: definitionDetail(d),
			})
		}
		// Then definitions imported from require'd modules.
		for _, d := range doc.external {
			if seen[d.name] {
				continue
			}
			seen[d.name] = true
			items = append(items, CompletionItem{
				Label:  d.name,
				Kind:   completionKindOf(d.kind),
				Detail: definitionDetail(d.definition) + " — " + filepath.Base(d.path),
			})
		}
	}

	// Then every symbol known to the interpreter environment (core
	// libraries, headers, …). VSCode does the prefix filtering.
	if s.env != nil {
		names := []string{}
		for _, r := range s.env.Symbols(nil, "") {
			name := string(r)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			items = append(items, CompletionItem{
				Label: name,
				Kind:  s.envSymbolKind(name),
			})
		}
	}
	s.respond(req, items)
}

func (s *Server) handleHover(req *requestMessage) {
	var p TextDocumentPositionParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.respondError(req, codeInvalidParams, err.Error())
		return
	}
	s.mu.Lock()
	doc := s.docs[p.TextDocument.URI]
	s.mu.Unlock()
	if doc == nil {
		s.respond(req, nil)
		return
	}
	sym := symbolAt(doc.content, p.Position.Line, p.Position.Character)
	if sym == "" {
		s.respond(req, nil)
		return
	}

	// Document-local definition wins: show its header.
	for _, d := range doc.analysis.defs {
		if d.name == sym {
			s.respond(req, Hover{Contents: MarkupContent{
				Kind:  "markdown",
				Value: "```lisp\n" + definitionDetail(d) + "\n```",
			}})
			return
		}
	}

	// Then definitions imported from require'd modules, with their
	// source file as context.
	for _, d := range doc.external {
		if d.name == sym {
			s.respond(req, Hover{Contents: MarkupContent{
				Kind:  "markdown",
				Value: "```lisp\n" + definitionDetail(d.definition) + "\n```\n" + filepath.Base(d.path),
			}})
			return
		}
	}

	// Otherwise ask the interpreter environment.
	if s.env != nil {
		if v, err := s.env.Get(types.Symbol{Val: sym}); err == nil {
			s.respond(req, Hover{Contents: MarkupContent{
				Kind:  "markdown",
				Value: fmt.Sprintf("```lisp\n%s\n```\n%s", sym, envValueDetail(v)),
			}})
			return
		}
	}
	s.respond(req, nil)
}

func (s *Server) handleDocumentSymbol(req *requestMessage) {
	var p DocumentSymbolParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.respondError(req, codeInvalidParams, err.Error())
		return
	}
	s.mu.Lock()
	doc := s.docs[p.TextDocument.URI]
	s.mu.Unlock()
	symbols := []DocumentSymbol{}
	if doc != nil {
		for _, d := range doc.analysis.defs {
			kind := symbolKindFunction
			if d.kind == "def" {
				kind = symbolKindVariable
			}
			symbols = append(symbols, DocumentSymbol{
				Name:           d.name,
				Detail:         definitionDetail(d),
				Kind:           kind,
				Range:          rangeOf(d.formPos),
				SelectionRange: symbolRange(d.namePos, d.name),
			})
		}
	}
	s.respond(req, symbols)
}

// completionKindOf maps a definition kind to a CompletionItem kind.
func completionKindOf(kind string) int {
	if kind == "def" {
		return completionKindVariable
	}
	return completionKindFunction
}

// definitionDetail renders a one-line header for a definition, e.g.
// `(defn add [a b])`.
func definitionDetail(d definition) string {
	if d.params != "" {
		return fmt.Sprintf("(%s %s %s)", d.kind, d.name, d.params)
	}
	return fmt.Sprintf("(%s %s)", d.kind, d.name)
}

// envSymbolKind classifies an environment symbol for completion.
func (s *Server) envSymbolKind(name string) int {
	v, err := s.env.Get(types.Symbol{Val: name})
	if err != nil {
		return completionKindVariable
	}
	switch v.(type) {
	case types.Func, types.MalFunc:
		return completionKindFunction
	}
	return completionKindVariable
}

// envValueDetail renders a short description of an environment value
// for hover.
func envValueDetail(v types.MalType) string {
	switch fn := v.(type) {
	case types.Func:
		return "builtin function"
	case types.MalFunc:
		if fn.IsMacro {
			return "macro"
		}
		return "function"
	default:
		return fmt.Sprintf("%T", v)
	}
}

// respond sends a successful JSON-RPC response. Requests without an ID
// (notifications) get no response per the spec.
func (s *Server) respond(req *requestMessage, result interface{}) {
	if req.ID == nil {
		return
	}
	_ = s.t.WriteMessage(responseMessage{JSONRPC: "2.0", ID: req.ID, Result: result})
}

// respondError sends a failed JSON-RPC response.
func (s *Server) respondError(req *requestMessage, code int, msg string) {
	if req.ID == nil {
		return
	}
	_ = s.t.WriteMessage(errorResponseMessage{
		JSONRPC: "2.0", ID: req.ID,
		Error: &responseError{Code: code, Message: msg},
	})
}

// notify sends a JSON-RPC notification to the client.
func (s *Server) notify(method string, params interface{}) {
	_ = s.t.WriteMessage(notificationMessage{JSONRPC: "2.0", Method: method, Params: params})
}
