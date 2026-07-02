;; test.lisp

;; this is a simple code to test the lisp-debugger
(do
    (def a 33)
    (println 1)
    (println 2)
    (println 3)
    (if (= 1 2)
        (println 4)
        (do
            (def b 44)
            (println b))
    ))