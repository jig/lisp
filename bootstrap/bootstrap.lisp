(def *host-language* "go")

(def not (fn (a)
    (if a false true)))

;; (def load-file (fn (f)
;;     (eval (read-string
;;         (str "(do " (slurp f) "\nnil)")))))

;; debug compatible version of load-file
(def load-file (fn (f)
    (eval (read-string
        (push-file-to-debug f (str "(do " (slurp f) "\nnil)"))))))


(defmacro cond (fn (& xs)
    (if (> (count xs) 0)
        (list
            'if
            (first xs)
            (if (> (count xs) 1) (nth xs 1) (throw "odd number of forms to cond"))
            (cons 'cond (rest (rest xs)))))))
