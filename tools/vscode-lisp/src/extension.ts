import * as vscode from "vscode";

/**
 * Activate the extension. Registers a DebugAdapterDescriptorFactory for
 * the `lisp` debug type. The factory tells VSCode to spawn the
 * lispdebug-build interpreter as the DAP server, talking over stdio.
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
}

export function deactivate(): void {
  /* no-op */
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
    const args = [...extra, "--dap", program];
    return new vscode.DebugAdapterExecutable(command, args);
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
    return config;
  }
}
