(def load-file (fn (f)
                        (eval
                            (read-string
                                (str ";; $MODULE " f "\n(do " (slurp f) " nil)")))))
