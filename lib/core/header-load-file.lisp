(def load-file (fn (f)
    (eval
        (read-string
            (str
                ";; $MODULE " (quote f) "\n(do " (slurp f) "\n)")))))