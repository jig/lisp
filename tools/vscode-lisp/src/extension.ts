import { execFile } from "child_process";
import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from "vscode-languageclient/node";
import { activateTesting } from "./tests";

let client: LanguageClient | undefined;

const installHint =
  "go install -tags debugger github.com/jig/lisp/cmd/lisp@latest";

type Probe = "ok" | "release-build" | "not-found";

/**
 * probeDebugBuild checks that `command --dap` gets past the build-tag
 * gate (a debug build proceeds to its own argument validation; a
 * release build refuses with "requires a debug build"). Lets us show
 * an actionable message instead of VSCode's generic "the debug
 * adapter exited unexpectedly".
 */
function probeDebugBuild(command: string): Promise<Probe> {
  return new Promise((resolve) => {
    execFile(command, ["--dap"], { timeout: 5000 }, (error, _stdout, stderr) => {
      const err = error as (Error & { code?: string }) | null;
      if (err && err.code === "ENOENT") {
        resolve("not-found");
        return;
      }
      if (typeof stderr === "string" && stderr.includes("requires a debug build")) {
        resolve("release-build");
        return;
      }
      resolve("ok");
    });
  });
}

/**
 * Activate the extension. Registers a DebugAdapterDescriptorFactory for
 * the `lisp` debug type (the factory tells VSCode to spawn the
 * lispdebug-build interpreter as the DAP server over stdio) and starts
 * the LSP client against the same binary (`lisp --lsp`).
 */
export function activate(context: vscode.ExtensionContext): void {
  context.subscriptions.push(
    vscode.debug.registerDebugAdapterDescriptorFactory(
      "lisp",
      new LispDebugAdapterDescriptorFactory(),
    ),
  );

  context.subscriptions.push(
    vscode.debug.registerDebugConfigurationProvider(
      "lisp",
      new LispDebugConfigurationProvider(),
    ),
  );

  activateTesting(context);

  const cfg = vscode.workspace.getConfiguration("lisp");
  if (cfg.get<boolean>("languageServer.enabled", true)) {
    const command = cfg.get<string>("languageServer.command", "lisp");
    // Spawn at the workspace root: require's git-root search walks up
    // from the process cwd.
    const cwd = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
    const serverOptions: ServerOptions = {
      command,
      args: ["--lsp"],
      options: { cwd },
    };
    const clientOptions: LanguageClientOptions = {
      documentSelector: [{ language: "lisp" }],
      initializationOptions: {
        includeDirs: cfg.get<string[]>("languageServer.includeDirs", []),
      },
      synchronize: {
        // Notify the server when any .lisp/.mal file changes on disk,
        // even if it is not open in an editor, so requiring documents
        // re-analyse against the new module content.
        fileEvents: vscode.workspace.createFileSystemWatcher("**/*.{lisp,mal}"),
      },
    };
    client = new LanguageClient(
      "lisp",
      "jig/lisp language server",
      serverOptions,
      clientOptions,
    );
    // start() spawns the server; errors (e.g. binary not on PATH) are
    // surfaced by the client in its output channel without breaking
    // the debugger features.
    void client.start();
    // A release-build binary dies instantly on --lsp; explain how to
    // fix it instead of leaving only the client's cryptic crash note.
    void probeDebugBuild(command).then((probe) => {
      if (probe === "release-build") {
        void vscode.window.showWarningMessage(
          `jig/lisp: '${command}' is a release build — the language server ` +
            `and debugger need a debug build. Reinstall with: ${installHint}`,
        );
      }
    });
  }
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}

class LispDebugAdapterDescriptorFactory
  implements vscode.DebugAdapterDescriptorFactory {
  async createDebugAdapterDescriptor(
    session: vscode.DebugSession,
    _executable: vscode.DebugAdapterExecutable | undefined,
  ): Promise<vscode.DebugAdapterDescriptor> {
    const cfg = vscode.workspace.getConfiguration("lisp");
    const command = cfg.get<string>("debugAdapter.command", "lisp");
    const extra = cfg.get<string[]>("debugAdapter.extraArgs", []);

    switch (await probeDebugBuild(command)) {
      case "not-found":
        throw new Error(
          `lisp debug: '${command}' not found on PATH — install it with: ${installHint}`,
        );
      case "release-build":
        throw new Error(
          `lisp debug: '${command}' is a release build without the debugger — ` +
            `reinstall with: ${installHint}`,
        );
    }

    const program = session.configuration.program;
    if (typeof program !== "string" || program.length === 0) {
      throw new Error(
        "lisp debug: 'program' is required in launch configuration",
      );
    }
    const args = [...extra];
    const preamble = session.configuration.preamble;
    if (Array.isArray(preamble)) {
      for (const assignment of preamble) {
        if (typeof assignment === "string" && assignment.length > 0) {
          args.push("--preamble", assignment);
        }
      }
    }
    // Debug Test: run just the named deftest after the script loads.
    const runTest = session.configuration.runTest;
    if (typeof runTest === "string" && runTest.length > 0) {
      args.push("--run-test", runTest);
    }
    args.push("--dap", program);
    const cfgEnv = session.configuration.env;
    const options: vscode.DebugAdapterExecutableOptions = {};
    if (cfgEnv && typeof cfgEnv === "object") {
      options.env = cfgEnv as { [key: string]: string };
    }
    return new vscode.DebugAdapterExecutable(command, args, options);
  }
}

class LispDebugConfigurationProvider
  implements vscode.DebugConfigurationProvider {
  /**
   * Fill in defaults when the user runs F5 without a launch.json.
   */
  resolveDebugConfiguration(
    _folder: vscode.WorkspaceFolder | undefined,
    config: vscode.DebugConfiguration,
    _token?: vscode.CancellationToken,
  ): vscode.ProviderResult<vscode.DebugConfiguration> {
    if (!config.type && !config.request && !config.name) {
      const editor = vscode.window.activeTextEditor;
      if (editor && editor.document.languageId === "lisp") {
        config.type = "lisp";
        config.name = "Launch";
        config.request = "launch";
        config.program = editor.document.fileName;
        config.stopOnEntry = true;
      }
    }
    if (!config.program) {
      return vscode.window
        .showInformationMessage(
          "Cannot find a program to debug. Open a .lisp file or set 'program' in launch.json.",
        )
        .then(() => undefined);
    }
    if (!config.cwd) {
      config.cwd = _folder?.uri.fsPath ??
        vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
    }
    return config;
  }
}
