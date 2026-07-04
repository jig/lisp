;; $MODULE header-basic

(do
    (def *host-language* "go")

    (def not (fn [a]
        (if a
            false
            true)))

    (defmacro cond (fn [& xs]
        (if (> (count xs) 0)
            (list
                'if (first xs)
                    (if (> (count xs) 1)
                        (nth xs 1)
                        (throw "odd number of forms to cond"))
                    (cons 'cond (rest (rest xs)))))))

    ;; (defn name [params] body...)
    ;; (defn name "docstring" [params] body...)   ; Clojure-style
    ;; A leading string between the name and the parameter vector is a
    ;; docstring, stored as {:doc "..."} metadata on the function (read
    ;; it with (meta f), (doc name), tooling, etc.).
    (defmacro defn (fn [name & fdecl]
        (if (string? (first fdecl))
            `(def ~name
                (with-meta
                    (fn ~(first (rest fdecl)) ~@(rest (rest fdecl)))
                    (hash-map :doc ~(first fdecl))))
            `(def ~name
                (fn ~(first fdecl) ~@(rest fdecl)))))))
