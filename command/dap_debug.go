//go:build debugger

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
func startDAP(listen, script string, preamble []string, runTest string, env types.EnvType) error {
	if script == "" {
		return fmt.Errorf("--dap requires a script positional argument (REPL mode not yet supported)")
	}
	abs, err := filepath.Abs(script)
	if err != nil {
		return fmt.Errorf("cannot resolve script path: %w", err)
	}
	registerModule(script, abs)

	eval := func(ctx context.Context, env types.EnvType) error {
		// runScript's classic path issues a `(load-file …)` bootstrap
		// call; the anonymous cursor (no module) makes the debugger
		// treat it as non-user code, so breakpoints and steps land on
		// the first real statement of the loaded file. With preamble
		// placeholders (from --preamble flags or in-file `;; $NAME`
		// lines) the file is evaluated directly with the same
		// `;; $MODULE` source mapping.
		if _, err := runScript(ctx, env, script, preamble, types.NewAnonymousCursorHere(1, 1)); err != nil {
			return err
		}
		// Debug Test: the script has registered its deftests; run the
		// requested one in this same session so breakpoints in its body
		// (which carry the script's positions) stop the debugger.
		if runTest != "" {
			runOne := types.NewList(nil, types.Symbol{Val: "test/run-test!"}, runTest)
			if _, err := lisp.EVAL(ctx, runOne, env); err != nil {
				return err
			}
		}
		return nil
	}

	// redirectStdout swaps os.Stdout for a pipe whose contents are
	// forwarded to the client as DAP `output` events, so println/prn
	// output shows up in the Debug Console. In stdio mode this also
	// keeps raw prints from corrupting the protocol framing. Returns a
	// restore function.
	redirectStdout := func(srv *debugadapter.Server) func() {
		r, w, err := os.Pipe()
		if err != nil {
			return func() {}
		}
		orig := os.Stdout
		os.Stdout = w
		go srv.StreamOutput(r, "stdout")
		return func() {
			os.Stdout = orig
			_ = w.Close()
		}
	}

	if listen == "" {
		// Capture the real stdout for the transport before it is
		// replaced by the output-forwarding pipe.
		t := debugadapter.NewTransport(os.Stdin, os.Stdout, nil)
		srv := debugadapter.NewServer(t, eval, env)
		defer redirectStdout(srv)()
		return srv.Run(context.Background())
	}

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listen, err)
	}
	defer func() { _ = ln.Close() }()
	fmt.Fprintf(os.Stderr, "DAP server listening on %s\n", listen)
	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("accept: %w", err)
	}
	defer func() { _ = conn.Close() }()
	t := debugadapter.NewTransport(conn, conn, conn)
	srv := debugadapter.NewServer(t, eval, env)
	defer redirectStdout(srv)()
	return srv.Run(context.Background())
}
