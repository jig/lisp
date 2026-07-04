;; A mal-in-mal interpreter (from kanaka/mal) adapted to jig/lisp.
;;
;; Differences from examples/mal.lisp, showcasing jig/lisp features:
;;   * top-level definitions use `defn` with Clojure-style docstrings
;;     (readable with `(doc name)` and surfaced by the LSP on hover)
;;   * the hosted core namespace (`core_ns`) exposes jig/lisp's extra
;;     builtins (range, merge, get-in, update, uuid, sets, …) so programs
;;     interpreted by this mini-mal can use them too
;;   * no surrounding `(do …)` wrapper: jig/lisp evaluates every
;;     top-level form in the file

;; ENV
;;
;;  An environment is an atom referencing a map where keys are strings
;;  instead of symbols.  The outer environment is the value associated
;;  with the normally invalid :outer key.

(defn bind-env
  "Bind parameter symbols b to values e on top of env; `&` binds the rest."
  [env b e]
  (if (empty? b)
    env
    (let [b0 (first b)]
      (if (= '& b0)
        (assoc env (str (nth b 1)) e)
        (bind-env (assoc env (str b0) (first e)) (rest b) (rest e))))))

(defn new-env
  ¬Create a child environment (atom) whose :outer is the first argument,
  optionally binding the given parameter/value sequences.¬
  [& args]
  (if (<= (count args) 1)
    (atom {:outer (first args)})
    (atom (apply bind-env {:outer (first args)} (rest args)))))

(defn env-find
  "Return the environment in which symbol k is bound, or nil."
  [env k]
  (env-find-str env (str k)))

(defn env-find-str
  "Like env-find but takes an already-stringified key; walks :outer."
  [env ks]
  (if env
    (let [data @env]
      (if (contains? data ks)
        env
        (env-find-str (get data :outer) ks)))))

(defn env-get
  "Look up symbol k following the :outer chain; throws if unbound."
  [env k]
  (let [ks (str k)
        e (env-find-str env ks)]
    (if e
      (get @e ks)
      (throw (str "'" ks "' not found")))))

(defn env-set
  "Define k = v in env and return v."
  [env k v]
  (do
    (swap! env assoc (str k) v)
    v))

;; CORE

(defn _macro?
  "Whether x is a macro marker map (created by defmacro below)."
  [x]
  (if (map? x)
    (contains? x :__MAL_MACRO__)
    false))

;; Host builtins made available inside the interpreted environment.
;; The first block matches kanaka/mal; the second exposes jig/lisp's
;; additions (every entry must be a plain function, not a macro or a
;; special form such as `future`, `context` or `try`).
(def core_ns
  '[* + - / < <= = > >= apply assoc atom atom? concat conj
    cons contains? count deref dissoc empty? false? first fn? get
    hash-map keys keyword keyword? list list? map map? meta nil?
    nth number? pr-str println prn read-string readline reset! rest seq
    sequential? slurp str string? swap! symbol symbol? throw time-ms
    true? vals vec vector vector? with-meta

    ;; --- lib/core extras (Go builtins beyond kanaka/mal) ---
    range merge get-in rename-keys assoc-in update update-in reduce-kv
    take take-last drop drop-last subvec split not= hash-set set set?
    json-encode json-decode hash-map-decode
    base64 unbase64 str2binary binary2str
    uuid sleep time-ns spew type? doc version assert
    go-error new-go-error new-error unwrap-error error-string panic

    ;; --- lib/coreextended (lisp-defined functions; its macros when, ->,
    ;; ->> and time are defined in the bootstrap below, as `(eval sym)`
    ;; cannot bind a macro; benchmark and defprotocol are left out) ---
    reduce foldr inc dec zero? identity partial gensym
    every? some memoize find-type extend satisfies? pprint

    ;; --- lib/system ---
    getenv setenv unsetenv

    ;; --- lib/require (resolve-require only; see note in main) ---
    resolve-require])

;; EVAL extends this stack trace-atom when propagating exceptions.  If the
;; exception reaches the REPL loop, the full trace-atom is printed.
(def trace-atom (atom ""))

;; read
(def READ read-string)

;; eval

(defn qq-loop
  "One step of quasiquote's right fold over a sequence element."
  [elt acc]
  (if (if (list? elt) (= (first elt) 'splice-unquote)) ; 2nd 'if' means 'and'
    (list 'concat (nth elt 1) acc)
    (list 'cons (QUASIQUOTE elt) acc)))

(defn qq-foldr
  "Right fold expanding the elements of a quasiquoted sequence."
  [xs]
  (if (empty? xs)
    ()
    (qq-loop (first xs) (qq-foldr (rest xs)))))

(defn QUASIQUOTE
  "Expand a quasiquoted AST into code that rebuilds it at run time."
  [ast]
  (cond
    (vector? ast) (list 'vec (qq-foldr ast))
    (map? ast) (list 'quote ast)
    (symbol? ast) (list 'quote ast)
    (not (list? ast)) ast
    (= (first ast) 'unquote) (nth ast 1)
    "else" (qq-foldr ast)))

(defn MACROEXPAND
  "Repeatedly expand ast while its head resolves to a macro."
  [ast env]
  (let [a0 (if (list? ast) (first ast))
        e (if (symbol? a0) (env-find env a0))
        m (if e (env-get e a0))]
    (if (_macro? m)
      (MACROEXPAND (apply (get m :__MAL_MACRO__) (rest ast)) env)
      ast)))

(defn eval-ast
  ¬Evaluate the sub-parts of ast: symbols, and the items of lists,
  vectors and maps.¬
  [ast env]
  ;; (do (prn "eval-ast" ast "/" (keys @env)) )
  (cond
    (symbol? ast) (env-get env ast)
    (list? ast) (map (fn [exp] (EVAL exp env)) ast)
    (vector? ast) (vec (map (fn [exp] (EVAL exp env)) ast))
    (map? ast) (apply hash-map
                 (apply concat
                   (map (fn [k] [k (EVAL (get ast k) env)])
                     (keys ast))))
    "else" ast))

(defn LET
  "Evaluate a let: bind the pairs in binds sequentially, then the body."
  [env binds form]
  (if (empty? binds)
    (EVAL form env)
    (do
      (env-set env (first binds) (EVAL (nth binds 1) env))
      (LET env (rest (rest binds)) form))))

(defn EVAL
  ¬Evaluate ast in env. The heart of the interpreter: handles the
  special forms and otherwise applies the evaluated head to its args.¬
  [ast env]
  ;; (do (prn "EVAL" ast "/" (keys @env)) )
  (try
    (let [ast (MACROEXPAND ast env)]
      (if (not (list? ast))
        (eval-ast ast env)

        ;; apply list
        (let [a0 (first ast)]
          (cond
            (empty? ast)
            ast

            (= 'def a0)
            (env-set env (nth ast 1) (EVAL (nth ast 2) env))

            (= 'let a0)
            (LET (new-env env) (nth ast 1) (nth ast 2))

            (= 'quote a0)
            (nth ast 1)

            (= 'quasiquoteexpand a0)
            (QUASIQUOTE (nth ast 1))

            (= 'quasiquote a0)
            (EVAL (QUASIQUOTE (nth ast 1)) env)

            (= 'defmacro a0)
            (env-set env (nth ast 1) (hash-map :__MAL_MACRO__
                                       (EVAL (nth ast 2) env)))

            (= 'macroexpand a0)
            (MACROEXPAND (nth ast 1) env)

            (= 'try a0)
            (if (< (count ast) 3)
              (EVAL (nth ast 1) env)
              (try
                (EVAL (nth ast 1) env)
                (catch exc
                  (do
                    (reset! trace-atom "")
                    (let [a2 (nth ast 2)]
                      (EVAL (nth a2 2) (new-env env [(nth a2 1)] [exc])))))))

            (= 'do a0)
            (nth (eval-ast (rest ast) env) (- (count ast) 2))

            (= 'if a0)
            (if (EVAL (nth ast 1) env)
              (EVAL (nth ast 2) env)
              (if (> (count ast) 3)
                (EVAL (nth ast 3) env)))

            (= 'fn a0)
            (fn [& args] (EVAL (nth ast 2) (new-env env (nth ast 1) args)))

            "else"
            (let [el (eval-ast ast env)]
              (apply (first el) (rest el)))))))

    (catch exc
      (do
        (swap! trace-atom str "\n  in mal EVAL: " ast)
        (throw exc)))))

;; print
(def PRINT pr-str)

;; repl
(def repl-env (new-env))

(defn rep
  "Read, evaluate and print one string, returning the printed result."
  [strng]
  (PRINT (EVAL (READ strng) repl-env)))

;; core.mal: defined directly using mal
(map (fn [sym] (env-set repl-env sym (eval sym))) core_ns)
(env-set repl-env 'macro? _macro?)
(env-set repl-env 'eval (fn [ast] (EVAL ast repl-env)))
(env-set repl-env '*ARGV* (rest *ARGV*))

;; core.mal: defined using the new language itself. These forms are
;; evaluated by EVAL in repl-env; passing quoted forms straight to EVAL
;; (rather than strings through READ) avoids escaping and a redundant
;; re-parse — READ here is the host reader anyway.
(defn mal-eval
  "Evaluate form in the interpreted (repl-env) environment."
  [form]
  (EVAL form repl-env))

(env-set repl-env '*host-language* (str *host-language* "-jig/lisp"))
(mal-eval '(def not (fn [a] (if a false true))))
(mal-eval '(def load-file
             (fn [f] (eval (read-string (str "(do " (slurp f) "\nnil)"))))))
(mal-eval '(defmacro cond
             (fn [& xs]
               (if (> (count xs) 0)
                 (list 'if (first xs)
                   (if (> (count xs) 1)
                     (nth xs 1)
                     (throw "odd number of forms to cond"))
                   (cons 'cond (rest (rest xs))))))))
;; defn: like def but with an optional leading docstring, mirroring
;; jig/lisp's own defn. The docstring is currently discarded.
(mal-eval '(defmacro defn
             (fn [name & fdecl]
               (if (string? (first fdecl))
                 (list 'def name (cons 'fn (rest fdecl)))
                 (list 'def name (cons 'fn fdecl))))))
;; or/and: this interpreter's own main uses (or …), so a self-hosted run
;; (lisp lisp.lisp -- lisp.lisp …) needs them defined here too. They bind
;; a temporary to avoid re-evaluating the tested expression.
(mal-eval '(defmacro or
             (fn [& xs]
               (if (empty? xs)
                 nil
                 (if (= 1 (count xs))
                   (first xs)
                   (list 'let (list 'or_ (first xs))
                     (list 'if 'or_ 'or_ (cons 'or (rest xs)))))))))
(mal-eval '(defmacro and
             (fn [& xs]
               (if (empty? xs)
                 true
                 (if (= 1 (count xs))
                   (first xs)
                   (list 'let (list 'and_ (first xs))
                     (list 'if 'and_ (cons 'and (rest xs)) 'and_)))))))
;; when and the threading macros -> / ->> from coreextended. Kept
;; self-contained (only list/cons/concat) and expanded step by step.
(mal-eval '(defmacro when
             (fn [condition & body]
               (list 'if condition (cons 'do body)))))
(mal-eval '(defmacro ->
             (fn [x & forms]
               (if (empty? forms)
                 x
                 (cons '->
                   (cons (if (list? (first forms))
                           (cons (first (first forms))
                             (cons x (rest (first forms))))
                           (list (first forms) x))
                     (rest forms)))))))
(mal-eval '(defmacro ->>
             (fn [x & forms]
               (if (empty? forms)
                 x
                 (cons '->>
                   (cons (if (list? (first forms))
                           (concat (first forms) (list x))
                           (list (first forms) x))
                     (rest forms)))))))
;; time: evaluate expr, print the elapsed milliseconds, return its value.
(mal-eval '(defmacro time
             (fn [expr]
               (list 'let (list 'start (list 'time-ms)
                            'ret expr
                            'end (list 'time-ms))
                 (list 'do
                   (list 'println "Elapsed time:"
                     (list '- 'end 'start) "msecs")
                   'ret)))))

;; repl loop
(defn repl-loop
  "Interactive loop: read a line, print its evaluation, repeat."
  [line]
  (if line
    (do
      (if (not (= "" line))
        (try
          (println (rep line))
          (catch exc
            (do
              (println "Uncaught exception:" exc @trace-atom)
              (reset! trace-atom "")))))
      (repl-loop (readline "lisp-user> ")))))

;; main
;;
;; Modes, chosen from *ARGV*:
;;   (none)              start the REPL
;;   -e/--eval EXPR      evaluate EXPR and print the result
;;   FILE                load FILE
;;
;; NOTE: the host `lisp` CLI consumes its own -e/--eval even when they
;; appear after the script name, so to reach THIS interpreter's options
;; stop host flag parsing with `--`:
;;   lisp examples/lisp.lisp -- -e '(+ 1 2)'
;;   lisp examples/lisp.lisp -- program.lisp arg1 arg2
(let [a0 (first *ARGV*)]
  (cond
    (empty? *ARGV*)
    (do
      ;; Print the banner once, directly, so its nil return value is not
      ;; echoed the way repl-loop would echo every evaluated line.
      (println (str "jig/lisp [" *host-language* "-jig/lisp]"))
      (repl-loop (readline "lisp-user> ")))

    (or (= a0 "-e") (= a0 "--eval"))
    (if (< (count *ARGV*) 2)
      (println "usage: -- -e EXPRESSION")
      ;; Return the value (don't print it): the host echoes a script's
      ;; result, so this prints exactly once, like the host's own -e.
      (EVAL (READ (nth *ARGV* 1)) repl-env))

    "else"
    (rep (str "(load-file \"" a0 "\")"))))
