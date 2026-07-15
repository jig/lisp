;; Replica of tests/step5_tco.mal: tail calls run in constant stack.

(deftest tco-self-recursive
  (def tco-sum2 (fn (n acc) (if (= n 0) acc (tco-sum2 (- n 1) (+ n acc)))))
  (is (= 55 (tco-sum2 10 0)))
  (is (= 50005000 (tco-sum2 10000 0))))

(deftest tco-mutually-recursive
  (def tco-foo (fn (n) (if (= n 0) 0 (tco-bar (- n 1)))))
  (def tco-bar (fn (n) (if (= n 0) 0 (tco-foo (- n 1)))))
  (is (= 0 (tco-foo 10000))))
