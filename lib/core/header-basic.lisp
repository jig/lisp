;; $MODULE header-basic

(do
    (def *host-language* "go")

    (def not (with-meta (fn [a]
        (if a
            false
            true))
        {:doc "Logical negation: false when a is truthy, true otherwise."}))

    (defmacro cond (with-meta (fn [& xs]
        (if (> (count xs) 0)
            (list
                'if (first xs)
                    (if (> (count xs) 1)
                        (nth xs 1)
                        (throw "odd number of forms to cond"))
                    (cons 'cond (rest (rest xs))))))
        {:doc "Takes test/expr pairs; yields the expr of the first truthy test, or nil."}))

    ;; (defn name [params] body...)
    ;; (defn name "docstring" [params] body...)   ; Clojure-style
    ;; A leading string between the name and the parameter vector is a
    ;; docstring, stored as {:doc "..."} metadata on the function (read
    ;; it with (meta f), (doc name), tooling, etc.).
    (defmacro defn (with-meta (fn [name & fdecl]
        (if (string? (first fdecl))
            `(def ~name
                (with-meta
                    (fn ~(first (rest fdecl)) ~@(rest (rest fdecl)))
                    (hash-map :doc ~(first fdecl))))
            `(def ~name
                (fn ~(first fdecl) ~@(rest fdecl)))))
        {:doc "Defines a named function (defn name [params] body…); an optional docstring may follow name."}))

    ;; Fundamental helpers and control-flow macros, promoted from
    ;; coreextended so every library header can rely on them with only
    ;; core loaded (their Clojure counterparts live in clojure.core).

    (defn inc
        "Returns x + 1."
        [x]
        (+ x 1))

    (defn dec
        "Returns x - 1."
        [x]
        (- x 1))

    (defmacro when
        (with-meta
            (fn [condition & body]
                `(if ~condition (do ~@body)))
            {:doc "Evaluates body in an implicit do when condition is truthy; otherwise nil."}))

    ;; "x1 x2 .. xn" is rewritten so each argument is evaluated at most
    ;; once and evaluation stops at the first nil/false.
    (defmacro and
        (with-meta
            (fn [& xs]
                (cond (empty? xs)      true
                      (= 1 (count xs)) (first xs)
                      true             (let (condvar (gensym))
                                        `(let (~condvar ~(first xs))
                                            (if ~condvar (and ~@(rest xs)) ~condvar)))))
            {:doc "Evaluates its arguments in order, returning the first falsey one, or the last (true with none)."}))

    ;; "x1 x2 .. xn" is rewritten as nested ifs so each argument is
    ;; evaluated at most once; without arguments, returns nil.
    (defmacro or
        (with-meta
            (fn [& xs]
                (if (< (count xs) 2)
                    (first xs)
                    (let [r (gensym)]
                        `(let (~r ~(first xs)) (if ~r ~r (or ~@(rest xs)))))))
            {:doc "Evaluates its arguments in order, returning the first truthy one, or nil."})))
