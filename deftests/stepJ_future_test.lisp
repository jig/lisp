;; Replica of tests/stepJ_future.mal: futures, deref and cancellation.

(deftest future-deref
  (is (= 2 (deref (future (+ 1 1)))))
  (is (= 2 @(future (+ 1 1))))
  (is (= 2 (deref (future (do (sleep 10) (+ 1 1))))))
  (def future-async-sum (future (do (sleep 10) (+ 1 1))))
  (is (= 2 @future-async-sum)))

(deftest future-simultaneous-and-predicates
  (def future-sum1 (future (do (sleep 10) (+ 1 1))))
  (def future-sum2 (future (do (sleep 10) (+ 2 2))))
  (is (future? future-sum1))
  (is (future? future-sum2))
  (is (= false (future? 2)))
  (is (= 2 @future-sum1))
  (is (= 4 (deref future-sum2)))
  (is (future-done? future-sum1))
  (is (future-done? future-sum2))
  (is (= false (future-cancelled? future-sum1)))
  (is (= false (future-cancelled? future-sum2)))
  ;; cancelling a completed future reports false
  (is (= false (future-cancel future-sum1)))
  (is (= false (future-cancel future-sum2))))

(deftest future-cancellation
  (def future-slow (future (do (sleep 5000) (+ 1 1))))
  (is (= true (future-cancel future-slow)))
  (is (future-cancelled? future-slow)))

(deftest future-error-propagates-on-deref
  (is (= :threw (try @(future (/ 1 0)) (catch e :threw))))
  (def future-bad (future (/ 1 0)))
  (is (= false (future-cancelled? future-bad)))
  (sleep 10)
  (is (future-done? future-bad))
  (is (= :threw (try @future-bad (catch e :threw))))
  (is (= false (future-cancelled? future-bad)))
  (is (future-done? future-bad)))

(deftest future-captures-environment
  (def future-a 2)
  (def future-b 3)
  (is (= 5 @(future (+ future-a future-b))))
  (def future-fa (future 2))
  (def future-fb (future 3))
  (is (= 5 @(future (+ @future-fa @future-fb)))))
