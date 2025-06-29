(def load-file (fn (f)
    (eval (read-string
        (push-file-to-debug f (str ";; $MODULE " f "\n(do " (slurp f) " nil)"))))))
