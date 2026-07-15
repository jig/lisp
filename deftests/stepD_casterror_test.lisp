;; Replica of tests/stepD_casterror.mal: numeric errors are catchable and
;; carry their message; go-error literals round-trip.

(deftest casterror-division-by-zero
  (is (= :threw (try (/ 0 0) (catch e :threw))))
  (is (= :threw (try (/ 1 0) (catch e :threw))))
  (is (= "«go-error \"division by zero\"»" (pr-str (try (/ 1 0) (catch e e)))))
  ;; try without catch still propagates
  (is (= :threw (try (try (/ 1 0)) (catch e :threw)))))

(deftest casterror-non-number
  (is (= :threw (try (+ 1 :hello) (catch e :threw))))
  (is (= :threw (try (+ 1 "hello") (catch e :threw)))))

;; The original's «go-error "simple error"» literal cases are omitted:
;; «…» reader literals need an environment to resolve their constructor,
;; and load-file's read-string deliberately runs without one (reading a
;; string must not execute constructor code).
(deftest casterror-go-error-value
  (is (= "go-error" (type? (try (/ 1 0) (catch e e))))))
