//go:build lispdebug

package command

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/jig/lisp/lsp"
	"github.com/jig/lisp/types"
)

// startLSP starts an LSP session. listen is empty for stdio, or a
// "host:port" TCP address. env supplies builtin symbols for completion
// and hover; the caller has already loaded the standard libraries.
func startLSP(listen string, env types.EnvType) error {
	if listen == "" {
		t := lsp.NewTransport(os.Stdin, os.Stdout, nil)
		srv := lsp.NewServer(t, env)
		return srv.Run(context.Background())
	}

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listen, err)
	}
	defer func() { _ = ln.Close() }()
	fmt.Fprintf(os.Stderr, "LSP server listening on %s\n", listen)
	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("accept: %w", err)
	}
	defer func() { _ = conn.Close() }()
	t := lsp.NewTransport(conn, conn, conn)
	srv := lsp.NewServer(t, env)
	return srv.Run(context.Background())
}
