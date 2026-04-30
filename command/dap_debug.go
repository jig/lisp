//go:build lispdebug

package command

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/jig/lisp"
	"github.com/jig/lisp/debugadapter"
	"github.com/jig/lisp/types"
)

// startDAP starts a DAP session. listen is empty for stdio, or a
// "host:port" TCP address. script is the program to debug; if empty,
// the session is REPL-style and waits for `evaluate` requests (not yet
// implemented at MVP).
func startDAP(listen, script string, env types.EnvType) error {
	if script == "" {
		return fmt.Errorf("--dap requires a script positional argument (REPL mode not yet supported)")
	}
	abs, err := filepath.Abs(script)
	if err != nil {
		return fmt.Errorf("cannot resolve script path: %w", err)
	}
	registerModule(script, abs)

	eval := func(ctx context.Context, env types.EnvType) error {
		_, err := lisp.REPL(ctx, env, `(load-file "`+script+`")`, types.NewCursorHere(script, -3, 1))
		return err
	}

	if listen == "" {
		t := debugadapter.NewTransport(os.Stdin, os.Stdout, nil)
		srv := debugadapter.NewServer(t, eval, env)
		return srv.Run(context.Background())
	}

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listen, err)
	}
	defer ln.Close()
	fmt.Fprintf(os.Stderr, "DAP server listening on %s\n", listen)
	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("accept: %w", err)
	}
	defer conn.Close()
	t := debugadapter.NewTransport(conn, conn, conn)
	srv := debugadapter.NewServer(t, eval, env)
	return srv.Run(context.Background())
}
