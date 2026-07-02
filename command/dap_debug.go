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
		// The `(load-file …)` call is bootstrap scaffolding, not part of
		// the user's program. Give it an anonymous cursor (no module) so
		// the debugger treats it as non-user code: a breakpoint or step
		// won't stop on this synthetic call, letting the session land on
		// the first real statement of the loaded file instead. The file's
		// own forms get their module from the `;; $MODULE` prefix that
		// load-file injects, so source mapping is unaffected.
		_, err := lisp.REPL(ctx, env, `(load-file "`+script+`")`, types.NewAnonymousCursorHere(1, 1))
		return err
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
	defer ln.Close()
	fmt.Fprintf(os.Stderr, "DAP server listening on %s\n", listen)
	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("accept: %w", err)
	}
	defer conn.Close()
	t := debugadapter.NewTransport(conn, conn, conn)
	srv := debugadapter.NewServer(t, eval, env)
	defer redirectStdout(srv)()
	return srv.Run(context.Background())
}
