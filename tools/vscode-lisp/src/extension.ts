import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from "vscode-languageclient/node";

let client: LanguageClient | undefined;

/**
 * Activate the extension. Registers a DebugAdapterDescriptorFactory for
 * the `lisp` debug type (the factory tells VSCode to spawn the
 * lispdebug-build interpreter as the DAP server over stdio) and starts
 * the LSP client against the same binary (`lisp-debug --lsp`).
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

  const cfg = vscode.workspace.getConfiguration("lisp");
  if (cfg.get<boolean>("languageServer.enabled", true)) {
    const command = cfg.get<string>("languageServer.command", "lisp-debug");
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
  }
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}

class LispDebugAdapterDescriptorFactory
  implements vscode.DebugAdapterDescriptorFactory {
  createDebugAdapterDescriptor(
    session: vscode.DebugSession,
    _executable: vscode.DebugAdapterExecutable | undefined,
  ): vscode.ProviderResult<vscode.DebugAdapterDescriptor> {
    const cfg = vscode.workspace.getConfiguration("lisp");
    const command = cfg.get<string>("debugAdapter.command", "lisp-debug");
    const extra = cfg.get<string[]>("debugAdapter.extraArgs", []);

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
