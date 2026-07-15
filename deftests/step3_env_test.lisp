;; Replica of tests/step3_env.mal: def, let and environment nesting.

(deftest env-def
  (def env-x 3)
  (is (= 3 env-x))
  (def env-x 4)
  (is (= 4 env-x))
  (def env-y (+ 1 7))
  (is (= 8 env-y)))

(deftest env-case-sensitive-symbols
  (def env-mynum 111)
  (def env-MYNUM 222)
  (is (= 111 env-mynum))
  (is (= 222 env-MYNUM)))

(deftest env-lookup-error-aborts-def
  (is (= :threw (try (abc 1 2 3) (catch e :threw))))
  (def env-w 123)
  (is (= :threw (try (def env-w (abc)) (catch e :threw))))
  (is (= 123 env-w)))

(deftest env-let
  (is (= 9 (let (z 9) z)))
  (is (= 9 (let (x 9) x)))
  (def env-x2 4)
  (is (= 4 (let (unused 9) env-x2)))
  (is (= 6 (let (z (+ 2 3)) (+ 1 z))))
  (is (= 12 (let (p (+ 2 3) q (+ 2 p)) (+ p q))))
  (def env-y2 (let (z 7) z))
  (is (= 7 env-y2))
  ;; several body expressions: the last one wins
  (is (= 10 (let (z 9 x 10 y 11) z y x)))
  (is (= nil (let (z 9)))))

(deftest env-outer-environment
  (def env-a 4)
  (is (= 9 (let (q 9) q)))
  (is (= 4 (let (q 9) env-a)))
  (is (= 4 (let (z 2) (let (q 9) env-a)))))

;; -------- Deferrable Functionality --------

(deftest env-let-vector-bindings
  (is (= 9 (let [z 9] z)))
  (is (= 12 (let [p (+ 2 3) q (+ 2 p)] (+ p q)))))

(deftest env-vector-evaluation
  (is (= [3 4 5 [6 7] 8] (let (a 5 b 6) [3 4 a [b 7] 8]))))

;; -------- Optional Functionality --------

(deftest env-let-last-assignment-wins
  (is (= 3 (let (x 2 x 3) x))))
