;; Replica of tests/stepE_merge_assert.mal: merge, assert and rename-keys.

(deftest merge-basics
  (are [expected expr] (= expected expr)
    {}      (merge {} {})
    {:a 1}  (merge {:a 1} {})
    {:a 1}  (merge {:a 1} {:a 1})
    ;; second takes precedence
    {:a 111} (merge {:a 1} {:a 111})
    {:a 1}   (merge {} {:a 1}))
  (is (= 111 (get (merge {:a 1 :b 2} {:a 111}) :a)))
  (is (= 1 (get (merge {:x 1 :y 2} {:y 3 :z 4}) :x)))
  (is (= 3 (get (merge {:x 1 :y 2} {:y 3 :z 4}) :y)))
  (is (= 4 (get (merge {:x 1 :y 2} {:y 3 :z 4}) :z))))

(deftest merge-nil-maps
  (is (= {:a 1} (merge nil {:a 1})))
  (is (= {:a 1} (merge {:a 1} nil)))
  (is (= nil (merge nil nil))))

(deftest merge-defs
  (def merge-m1 {:a 1})
  (def merge-m2 {:a 2})
  (is (= {:a 2} (merge merge-m1 merge-m2)))
  (is (= {:m1 {:a 2}} (merge {:m1 merge-m1} {:m1 merge-m2}))))

(deftest assert-basics
  (is (= :threw (try (assert) (catch e :threw))))
  ;; only nil and false fail an assert
  (are [expr] (= nil expr)
    (assert true)
    (assert 0)
    (assert 1)
    (assert [1 2 3])
    (assert ())
    (assert {}))
  (is (= :threw (try (assert nil) (catch e :threw))))
  (is (= :threw (try (assert false) (catch e :threw)))))

(deftest assert-with-message
  (is (= nil (assert true "boom!")))
  ;; a string message is thrown wrapped as a go-error; other values raw
  (is (= "«go-error \"boom!\"»" (pr-str (try (assert nil "boom!") (catch e e)))))
  (is (= "«go-error \"boom!\"»" (pr-str (try (assert false "boom!") (catch e e)))))
  (is (= 3 (try (assert nil 3) (catch e e))))
  (is (= [3] (try (assert nil [3]) (catch e e))))
  (is (= (quote (3)) (try (assert nil (quote (3))) (catch e e))))
  (is (= {:3 3} (try (assert nil {:3 3}) (catch e e))))
  (is (= (quote assert) (try (assert nil (quote assert)) (catch e e)))))

(deftest rename-keys-basics
  (are [expected expr] (= expected expr)
    {}         (rename-keys {} {})
    {:a 1}     (rename-keys {:a 1} {})
    {"mimi" 1} (rename-keys {:a 1} {:a "mimi"}))
  (is (= 1 (get (rename-keys {:a 1 :b 3} {:a "mimi"}) "mimi")))
  (is (= 3 (get (rename-keys {:a 1 :b 3} {:a "mimi"}) :b))))
