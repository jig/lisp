;; Replica of tests/step4_fn_extra.mal: arity errors must not panic.

(deftest fn-arity-errors
  (def fn-extra-1 (fn [x] nil))
  (is (= nil (fn-extra-1 3)))
  (is (= :threw (try (fn-extra-1) (catch e :threw))))
  (def fn-extra-2 (fn [x y] nil))
  (is (= :threw (try (fn-extra-2 3) (catch e :threw))))
  (is (= :threw (try (fn-extra-2) (catch e :threw))))
  (is (= :threw (try (fn-extra-2 1 2 3) (catch e :threw)))))
