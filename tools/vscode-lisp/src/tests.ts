import * as vscode from "vscode";
import { execFile, ChildProcess } from "child_process";
import * as path from "path";
import * as os from "os";
import * as fs from "fs";

/**
 * Test integration: discovers (deftest …) forms in *_test.lisp / *_test.mal
 * files, runs them through the CLI runner (`lisp-debug --test FILE
 * --test-json REPORT`), and — for the Coverage profile — collects an lcov
 * report (`--coverage`) surfaced through VS Code's native test coverage
 * API (gutters, Test Coverage view).
 */

const DEFTEST_RE = /\(deftest\s+([^\s()[\]{}"']+)/g;
const TEST_GLOB = "**/*_test.{lisp,mal}";

interface CheckReport {
  form: string;
  ok: boolean;
  module?: string;
  line?: number;
  error?: string;
  expected?: string;
  actual?: string;
  message?: string;
}

interface TestReport {
  name: string;
  ok: boolean;
  module?: string;
  line?: number;
  error?: string;
  checks: CheckReport[];
}

interface SuiteReport {
  tests: TestReport[];
}

export function activateTesting(context: vscode.ExtensionContext): void {
  const ctrl = vscode.tests.createTestController("jigLispTests", "jig/lisp tests");
  context.subscriptions.push(ctrl);

  // Detailed per-line coverage is stashed here when a run produces it;
  // loadDetailedCoverage hands it back to VS Code lazily.
  const coverageDetails = new WeakMap<vscode.FileCoverage, vscode.StatementCoverage[]>();

  async function parseTestFile(uri: vscode.Uri): Promise<void> {
    let text: string;
    try {
      text = new TextDecoder().decode(await vscode.workspace.fs.readFile(uri));
    } catch {
      ctrl.items.delete(uri.toString());
      return;
    }
    const fileItem =
      ctrl.items.get(uri.toString()) ??
      ctrl.createTestItem(uri.toString(), path.basename(uri.fsPath), uri);
    fileItem.canResolveChildren = false;
    const children: vscode.TestItem[] = [];
    const lines = text.split("\n");
    for (let i = 0; i < lines.length; i++) {
      DEFTEST_RE.lastIndex = 0;
      let m: RegExpExecArray | null;
      while ((m = DEFTEST_RE.exec(lines[i])) !== null) {
        const item = ctrl.createTestItem(`${uri.toString()}#${m[1]}`, m[1], uri);
        item.range = new vscode.Range(i, 0, i, lines[i].length);
        children.push(item);
      }
    }
    if (children.length === 0) {
      ctrl.items.delete(uri.toString());
      return;
    }
    fileItem.children.replace(children);
    ctrl.items.add(fileItem);
  }

  async function discoverAll(): Promise<void> {
    const uris = await vscode.workspace.findFiles(TEST_GLOB);
    await Promise.all(uris.map(parseTestFile));
  }

  ctrl.resolveHandler = async (item) => {
    if (!item) {
      await discoverAll();
    }
  };

  const watcher = vscode.workspace.createFileSystemWatcher(TEST_GLOB);
  context.subscriptions.push(watcher);
  watcher.onDidCreate(parseTestFile);
  watcher.onDidChange(parseTestFile);
  watcher.onDidDelete((uri) => ctrl.items.delete(uri.toString()));

  async function runHandler(
    request: vscode.TestRunRequest,
    token: vscode.CancellationToken,
    withCoverage: boolean,
  ): Promise<void> {
    const run = ctrl.createTestRun(request);
    const fileItems = new Map<string, { file: vscode.TestItem; tests?: Set<string> }>();

    const addFile = (file: vscode.TestItem, test?: vscode.TestItem) => {
      let entry = fileItems.get(file.id);
      if (!entry) {
        entry = { file, tests: test ? new Set() : undefined };
        fileItems.set(file.id, entry);
      }
      if (test && entry.tests) {
        entry.tests.add(test.label);
      } else if (!test) {
        entry.tests = undefined; // whole file requested
      }
    };
    if (request.include) {
      for (const item of request.include) {
        if (item.parent) {
          addFile(item.parent, item);
        } else {
          addFile(item);
        }
      }
    } else {
      await discoverAll();
      ctrl.items.forEach((item) => addFile(item));
    }

    const cfg = vscode.workspace.getConfiguration("lisp");
    const command = cfg.get<string>("testRunner.command") ||
      cfg.get<string>("debugAdapter.command", "lisp-debug");

    for (const { file, tests } of fileItems.values()) {
      if (token.isCancellationRequested) {
        break;
      }
      const uri = file.uri;
      if (!uri) {
        continue;
      }
      const tmpBase = path.join(os.tmpdir(), `lisp-test-${Date.now()}-${Math.random().toString(36).slice(2)}`);
      const reportPath = `${tmpBase}.json`;
      const lcovPath = `${tmpBase}.lcov`;
      const args = ["--test", uri.fsPath, "--test-json", reportPath];
      if (withCoverage) {
        args.push("--coverage", lcovPath);
      }
      file.children.forEach((t) => {
        if (!tests || tests.has(t.label)) {
          run.enqueued(t);
        }
      });
      const { stdout, stderr } = await execRunner(command, args, path.dirname(uri.fsPath), token);
      if (stdout) {
        run.appendOutput(stdout.replace(/\n/g, "\r\n"));
      }
      if (stderr) {
        run.appendOutput(stderr.replace(/\n/g, "\r\n"));
      }

      let report: SuiteReport | undefined;
      try {
        report = JSON.parse(fs.readFileSync(reportPath, "utf8")) as SuiteReport;
      } catch {
        // No report: the runner failed before running tests (load error,
        // missing binary…) — mark everything errored with the output.
        file.children.forEach((t) => {
          if (!tests || tests.has(t.label)) {
            run.errored(t, new vscode.TestMessage(stderr || stdout || "test runner produced no report"));
          }
        });
      } finally {
        fs.rmSync(reportPath, { force: true });
      }

      if (report) {
        const byName = new Map(report.tests.map((t) => [t.name, t]));
        file.children.forEach((t) => {
          if (tests && !tests.has(t.label)) {
            return;
          }
          const res = byName.get(t.label);
          if (!res) {
            run.skipped(t);
            return;
          }
          run.started(t);
          if (res.ok) {
            run.passed(t);
            return;
          }
          const messages: vscode.TestMessage[] = [];
          if (res.error) {
            messages.push(new vscode.TestMessage(res.error));
          }
          for (const c of res.checks) {
            if (c.ok) {
              continue;
            }
            let msg: vscode.TestMessage;
            if (c.expected !== undefined && c.actual !== undefined && !c.error) {
              msg = vscode.TestMessage.diff(
                c.message ? `${c.form} — ${c.message}` : c.form,
                c.expected,
                c.actual,
              );
            } else {
              const text = c.error ? `${c.form}: ${c.error}` : `${c.form}: failed`;
              msg = new vscode.TestMessage(c.message ? `${text} — ${c.message}` : text);
            }
            const loc = checkLocation(uri, c);
            if (loc) {
              msg.location = loc;
            }
            messages.push(msg);
          }
          run.failed(t, messages);
        });
      }

      if (withCoverage) {
        try {
          for (const fc of parseLcov(lcovPath, coverageDetails)) {
            run.addCoverage(fc);
          }
        } catch {
          // no coverage output (e.g. release binary): ignore
        } finally {
          fs.rmSync(lcovPath, { force: true });
        }
      }
    }
    run.end();
  }

  ctrl.createRunProfile(
    "Run",
    vscode.TestRunProfileKind.Run,
    (req, token) => runHandler(req, token, false),
    true,
  );
  const covProfile = ctrl.createRunProfile(
    "Coverage",
    vscode.TestRunProfileKind.Coverage,
    (req, token) => runHandler(req, token, true),
    false,
  );
  covProfile.loadDetailedCoverage = async (_run, fileCoverage) =>
    coverageDetails.get(fileCoverage as vscode.FileCoverage) ?? [];
}

/** Resolve a check's module/line to a VS Code location, if possible. */
function checkLocation(testUri: vscode.Uri, c: CheckReport): vscode.Location | undefined {
  if (!c.line || c.line <= 0) {
    return undefined;
  }
  let file = c.module;
  if (!file) {
    return undefined;
  }
  if (!path.isAbsolute(file)) {
    file = path.join(path.dirname(testUri.fsPath), file);
  }
  const pos = new vscode.Position(c.line - 1, 0);
  return new vscode.Location(vscode.Uri.file(file), pos);
}

/** Run the CLI test runner; resolves with its output even on non-zero exit
 * (test failures exit 1 by design). */
function execRunner(
  command: string,
  args: string[],
  cwd: string,
  token: vscode.CancellationToken,
): Promise<{ stdout: string; stderr: string }> {
  return new Promise((resolve) => {
    let child: ChildProcess | undefined;
    const done = (stdout: string, stderr: string) => resolve({ stdout, stderr });
    child = execFile(command, args, { cwd }, (error, stdout, stderr) => {
      if (error && (error as NodeJS.ErrnoException).code === "ENOENT") {
        done("", `cannot run '${command}': not found on PATH`);
        return;
      }
      done(stdout ?? "", stderr ?? "");
    });
    token.onCancellationRequested(() => child?.kill());
  });
}

/** Parse an lcov tracefile into FileCoverage objects, stashing per-line
 * detail for loadDetailedCoverage. */
function parseLcov(
  lcovPath: string,
  details: WeakMap<vscode.FileCoverage, vscode.StatementCoverage[]>,
): vscode.FileCoverage[] {
  const text = fs.readFileSync(lcovPath, "utf8");
  const out: vscode.FileCoverage[] = [];
  let file: string | undefined;
  let statements: vscode.StatementCoverage[] = [];
  const flush = () => {
    if (!file) {
      return;
    }
    const covered = statements.filter((s) => (s.executed as number) > 0).length;
    const fc = new vscode.FileCoverage(
      vscode.Uri.file(file),
      new vscode.TestCoverageCount(covered, statements.length),
    );
    details.set(fc, statements);
    out.push(fc);
    file = undefined;
    statements = [];
  };
  for (const line of text.split("\n")) {
    if (line.startsWith("SF:")) {
      flush();
      file = line.slice(3).trim();
    } else if (line.startsWith("DA:")) {
      const [lineNo, count] = line.slice(3).split(",").map(Number);
      if (Number.isFinite(lineNo) && lineNo > 0) {
        statements.push(
          new vscode.StatementCoverage(
            Number.isFinite(count) ? count : 0,
            new vscode.Position(lineNo - 1, 0),
          ),
        );
      }
    } else if (line.startsWith("end_of_record")) {
      flush();
    }
  }
  flush();
  return out;
}
