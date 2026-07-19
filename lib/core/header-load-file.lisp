;; $MODULE header-load-file

(defn load-file
    "Reads and evaluates the lisp file at file-path in the current environment; returns the value of its last form."
    [file-path]
    (eval (read-program (slurp-source file-path) file-path)))

;; load-file-once is registered in Go by nscore.LoadInput: its seen-set
;; needs mutable state, and atoms live in the concurrent library, which
;; is not available at core-load time.
