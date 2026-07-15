;; Replica of tests/step2_eval.mal in the deftest framework.
;;
;; Conventions used throughout deftests/: assertions compare VALUES with
;; (is (= expected actual)) — the legacy ;=> harness compares printed
;; forms instead, so both suites remain valuable. Error expectations
;; (;/regex on stdout) translate to a (try … (catch e …)) pattern;
;; pure stdout-printing cases stay in the legacy harness only.

;; Testing evaluation of arithmetic operations
(deftest eval-arithmetic
  (is (= 3 (+ 1 2)))
  (are [expected expr] (= expected expr)
    11   (+ 5 (* 2 3))
    8    (- (+ 5 (* 2 3)) 3)
    2    (/ (- (+ 5 (* 2 3)) 3) 4)
    1010 (/ (- (+ 515 (* 87 311)) 302) 27)
    -18  (* -3 6)
    -994 (/ (- (+ 515 (* -87 311)) 296) 27)))

;; This should throw an error with no return value
(deftest eval-unknown-symbol-throws
  (is (= :threw (try (abc 1 2 3) (catch e :threw)))))

;; Testing empty list
(deftest eval-empty-list
  (is (list? ()))
  (is (empty? ()))
  (is (= "()" (pr-str ()))))

;; -------- Deferrable Functionality --------

;; Testing evaluation within collection literals
(deftest eval-inside-collection-literals
  (is (= [1 2 3] [1 2 (+ 1 2)]))
  (is (= {"a" 15} {"a" (+ 7 8)}))
  (is (= {:a 15} {:a (+ 7 8)}))
  (is (= 15 (get {:a (+ 7 8)} :a))))

;; Check that evaluation hasn't broken empty collections
(deftest eval-empty-collections
  (is (vector? []))
  (is (= 0 (count [])))
  (is (= "[]" (pr-str [])))
  (is (map? {}))
  (is (= 0 (count (keys {}))))
  (is (= "{}" (pr-str {}))))
