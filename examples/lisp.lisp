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
(def core_ns '[* + - / < <= = > >= apply assoc atom atom? concat conj
  cons contains? count deref dissoc empty? false? first fn? get
  hash-map keys keyword keyword? list list? map map? meta nil?
  nth number? pr-str println prn read-string readline reset! rest seq
  sequential? slurp str string? swap! symbol symbol? throw time-ms
  true? vals vec vector vector? with-meta

  ;; jig/lisp extras
  range merge get-in rename-keys assoc-in update update-in
  reduce reduce-kv foldr inc dec identity partial gensym
  take take-last drop drop-last subvec split uuid sleep spew type?
  go-error panic getenv setenv unsetenv set set?])

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
    (vector? ast)            (list 'vec (qq-foldr ast))
    (map? ast)               (list 'quote ast)
    (symbol? ast)            (list 'quote ast)
    (not (list? ast))        ast
    (= (first ast) 'unquote) (nth ast 1)
    "else"                   (qq-foldr ast)))

(defn MACROEXPAND
  "Repeatedly expand ast while its head resolves to a macro."
  [ast env]
  (let [a0 (if (list? ast) (first ast))
        e  (if (symbol? a0) (env-find env a0))
        m  (if e (env-get e a0))]
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
    (list? ast)   (map (fn [exp] (EVAL exp env)) ast)
    (vector? ast) (vec (map (fn [exp] (EVAL exp env)) ast))
    (map? ast)    (apply hash-map
                    (apply concat
                      (map (fn [k] [k (EVAL (get ast k) env)])
                           (keys ast))))
    "else"        ast))

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

;; core.mal: defined using the new language itself
(rep (str "(def *host-language* \"" *host-language* "-jig-lisp\")"))
(rep "(def not (fn [a] (if a false true)))")
(rep ¬(def load-file (fn (f) (eval (read-string (str "(do " (slurp f) "\nnil)")))))¬)
(rep "(defmacro cond (fn (& xs) (if (> (count xs) 0) (list 'if (first xs) (if (> (count xs) 1) (nth xs 1) (throw \"odd number of forms to cond\")) (cons 'cond (rest (rest xs)))))))")
;; defn: like def but with an optional leading docstring, mirroring
;; jig/lisp's own defn. The docstring is currently discarded.
(rep "(defmacro defn (fn (name & fdecl) (if (string? (first fdecl)) (list 'def name (cons 'fn (rest fdecl))) (list 'def name (cons 'fn fdecl)))))")

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
(if (empty? *ARGV*)
  (do
    ;; Print the banner once, directly, so its nil return value is not
    ;; echoed the way repl-loop would echo every evaluated line.
    (println (str "jig/lisp [" *host-language* "-jig-lisp]"))
    (repl-loop (readline "lisp-user> ")))
  (rep (str "(load-file \"" (first *ARGV*) "\")")))
