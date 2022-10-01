(loop [x 10 acc 0]
    (do
        (prn x acc)
        (if (= x 0)
            acc
            (recur (- x 1) (+ x acc)))))
