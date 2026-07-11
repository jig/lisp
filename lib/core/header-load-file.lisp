;; $MODULE header-load-file

(defn load-file
    "Reads and evaluates the lisp file at file-path in the current environment; returns the value of its last form."
    [file-path]
    (eval
        (read-string
            (str
                ";; $MODULE " file-path "\n"
                "(do " (slurp file-path) "\n)"))))
