;; Replica of tests/step9_try.mal: try/catch/finally, apply, map,
;; predicates, hash-maps and the issue-#87 regressions. Cases asserting
;; prn output stay in the legacy harness.

(deftest try-catch-basics
  (is (= 123 (try 123 (catch e 456))))
  (is (= :caught (try abc (catch exc :caught))))
  (is (= :caught (try (abc 1 2) (catch exc :caught))))
  ;; errors from core builtins can be caught
  (is (= :caught (try (nth () 1) (catch exc :caught))))
  (is (= 7 (try (throw "my exception") (catch exc (do (str "exc:" exc) 7))))))

(deftest try-handlers-restore
  (is (= "c2" (try (do (try "t1" (catch e "c1")) (throw "e1")) (catch e "c2"))))
  (is (= "c2" (try (try (throw "e1") (catch e (throw "e2"))) (catch e "c2")))))

(deftest throw-is-a-function
  (is (= "my err" (try (map throw (list "my err")) (catch exc exc)))))

(deftest type-predicates
  (are [expected expr] (= expected expr)
    true  (symbol? (quote abc))
    false (symbol? "abc")
    true  (nil? nil)
    false (nil? true)
    true  (true? true)
    false (true? false)
    false (true? true?)
    true  (false? false)
    false (false? true)))

(deftest apply-function
  (are [expected expr] (= expected expr)
    5    (apply + (list 2 3))
    9    (apply + 4 (list 5))
    ()   (apply list (list))
    true (apply symbol? (list (quote two)))
    5    (apply (fn (a b) (+ a b)) (list 2 3))
    9    (apply (fn (a b) (+ a b)) 4 (list 5))
    9    (apply + 4 [5])
    ()   (apply list [])
    5    (apply (fn (a b) (+ a b)) [2 3])
    9    (apply (fn (a b) (+ a b)) 4 [5])))

(deftest map-function
  (def try-nums (list 1 2 3))
  (def try-double (fn (a) (* 2 a)))
  (is (= 6 (try-double 3)))
  (is (= (quote (2 4 6)) (map try-double try-nums)))
  (is (= (quote (false true false)) (map (fn (x) (symbol? x)) (list 1 (quote two) "three"))))
  (is (= () (map str ())))
  (is (= (quote (2 4 6)) (map (fn (a) (* 2 a)) [1 2 3])))
  (is (= (quote (true true)) (map (fn [& args] (list? args)) [1 2]))))

;; -------- Deferrable Functionality --------

(deftest symbol-keyword-constructors
  (are [expected expr] (= expected expr)
    false (symbol? :abc)
    true  (symbol? (quote abc))
    true  (symbol? (symbol "abc"))
    true  (keyword? :abc)
    false (keyword? (quote abc))
    false (keyword? "abc")
    false (keyword? "")
    true  (keyword? (keyword "abc")))
  (is (= (quote abc) (symbol "abc")))
  (is (= :abc (keyword "abc"))))

(deftest sequential-predicate
  (are [expected expr] (= expected expr)
    true  (sequential? (list 1 2 3))
    true  (sequential? [15])
    false (sequential? sequential?)
    false (sequential? nil)
    false (sequential? "abc")))

(deftest vector-constructors
  (are [expected expr] (= expected expr)
    true  (vector? [10 11])
    false (vector? (quote (12 13)))
    [3 4 5] (vector 3 4 5)
    true  (= [] (vector))
    true  (map? {})
    false (map? (quote ()))
    false (map? [])
    false (map? (quote abc))
    false (map? :abc)))

(deftest hash-map-basics
  (is (= {"a" 1} (hash-map "a" 1)))
  (is (= {"a" 1} {"a" 1}))
  (is (= {"a" 1} (assoc {} "a" 1)))
  (is (= 1 (get (assoc (assoc {"a" 1} "b" 2) "c" 3) "a")))
  (def try-hm1 (hash-map))
  (is (= {} try-hm1))
  (is (map? try-hm1))
  (is (= false (map? 1)))
  (is (= false (map? "abc")))
  (is (= nil (get nil "a")))
  (is (= nil (get try-hm1 "a")))
  (is (= false (contains? try-hm1 "a")))
  (def try-hm2 (assoc try-hm1 "a" 1))
  (is (= {"a" 1} try-hm2))
  ;; the original map is unchanged
  (is (= nil (get try-hm1 "a")))
  (is (= false (contains? try-hm1 "a")))
  (is (= 1 (get try-hm2 "a")))
  (is (contains? try-hm2 "a")))

(deftest hash-map-keys-vals
  (def try-hm1b (hash-map))
  (def try-hm2b (assoc try-hm1b "a" 1))
  (is (= () (keys try-hm1b)))
  (is (= (quote ("a")) (keys try-hm2b)))
  (is (= (quote ("1")) (keys {"1" 1})))
  (is (= () (vals try-hm1b)))
  (is (= (quote (1)) (vals try-hm2b)))
  (is (= 3 (count (keys (assoc try-hm2b "b" 2 "c" 3))))))

(deftest hash-map-keyword-keys
  (is (= 123 (get {:abc 123} :abc)))
  (is (contains? {:abc 123} :abc))
  (is (= false (contains? {:abcd 123} :abc)))
  (is (= {:bcd 234} (assoc {} :bcd 234)))
  (is (keyword? (nth (keys {:abc 123 :def 456}) 0)))
  (is (keyword? (nth (vals {"a" :abc "b" :def}) 0))))

(deftest hash-map-assoc-updates
  (def try-hm4 (assoc {:a 1 :b 2} :a 3 :c 1))
  (are [expected key] (= expected (get try-hm4 key))
    3 :a
    2 :b
    1 :c))

(deftest hash-map-nil-values
  (is (contains? {:abc nil} :abc))
  (is (= {:bcd nil} (assoc {} :bcd nil))))

(deftest str-pr-str-on-maps
  (is (= "A{:abc val}Z" (str "A" {:abc "val"} "Z")))
  (is (= "true.false.nil.:keyw.symb" (str true "." false "." nil "." :keyw "." (quote symb))))
  (is (= "\"A\" {:abc \"val\"} \"Z\"" (pr-str "A" {:abc "val"} "Z")))
  (is (= "true \".\" false \".\" nil \".\" :keyw \".\" symb" (pr-str true "." false "." nil "." :keyw "." (quote symb))))
  ;; multi-key print order is unstable: accept either
  (def try-s (str {:abc "val1" :def "val2"}))
  (is (or (= try-s "{:abc val1 :def val2}") (= try-s "{:def val2 :abc val1}")))
  (def try-p (pr-str {:abc "val1" :def "val2"}))
  (is (or (= try-p "{:abc \"val1\" :def \"val2\"}") (= try-p "{:def \"val2\" :abc \"val1\"}"))))

(deftest apply-with-mal-list-args
  (is (= true (apply (fn (& more) (list? more)) [1 2 3])))
  (is (= true (apply (fn (& more) (list? more)) [])))
  (is (= true (apply (fn (a & more) (list? more)) [1]))))

;; -------- Optional Functionality --------

(deftest throw-non-strings
  (is (= {:msg "err2"} (try (throw {:msg "err2"}) (catch e e))))
  (is (= 7 (try (throw (list 1 2 3)) (catch exc (do (str "err:" exc) 7)))))
  ;; try without catch propagates
  (is (= :threw (try (try xyz) (catch e :threw)))))

(deftest hash-map-dissoc
  (def try-hm3 (assoc (assoc (hash-map) "a" 1) "b" 2))
  (is (= 2 (count (keys try-hm3))))
  (is (= 2 (count (vals try-hm3))))
  (is (= {"b" 2} (dissoc try-hm3 "a")))
  (is (= {} (dissoc try-hm3 "a" "b")))
  (is (= {} (dissoc try-hm3 "a" "b" "c")))
  (is (= 2 (count (keys try-hm3))))
  (is (= {:fgh 456} (dissoc {:cde 345 :fgh 456} :cde)))
  (is (= {:fgh 456} (dissoc {:cde nil :fgh 456} :cde))))

(deftest hash-map-equality
  (are [expected expr] (= expected expr)
    true  (= {} {})
    true  (= {} (hash-map))
    true  (= {:a 11 :b 22} (hash-map :b 22 :a 11))
    false (= {:a 12 :b 22} (hash-map :b 22 :a 11))
    false (= {:c 11 :b 22} (hash-map :b 22 :a 11))
    true  (= {:a 11 :b [22 33]} (hash-map :b [22 33] :a 11))
    true  (= {:a 11 :b {:c 33}} (hash-map :b {:c 33} :a 11))
    false (= {:a 11 :b 22} (hash-map :b 23 :a 11))
    false (= {:a 11 :b 22} (hash-map :a 11))
    true  (= {:a [11 22]} {:a (list 11 22)})
    false (= {:a 11 :b 22} (list :a 11 :b 22))
    false (= {} [])
    false (= [] {})))

(deftest try-does-not-require-do
  (is (= 7 (try 1 2 (let [x 3] x) (throw "my exception") (catch exc 4 (let [x 5] x) 6 7))))
  (is (= true (try 1 2 3 true))))

(deftest try-catch-finally
  (def try-fin (atom false))
  (is (= 123 (try 123 (catch e 456) (finally (reset! try-fin true)))))
  (is (= 1 (try (let [x 1] x) (finally (reset! try-fin false)))))
  (is (= 2 (try (let [x 1] x) (let [x 2] x) (finally (reset! try-fin true)))))
  ;; finally cannot throw errors
  (is (= 2 (try (let [x 1] x) (let [x 2] x) (finally (throw "poum!"))))))

(deftest rethrow-from-catch
  (is (= :outer (try (try (throw 2) (catch err (throw 3))) (catch e :outer))))
  (is (= 3 (try (try (throw 2) (catch err (throw 3))) (catch err err))))
  (is (= :caught (try (/ 0 0) (catch e :caught))))
  (is (= 2 (try (throw 1) (catch e (+ e 1)))))
  (is (= "err:2" (try (throw 2) (catch e (str "err:" e))))))

;; The catch result is a value, not code (issue #87)
(deftest catch-result-not-reevaluated
  (is (= {:v (quote (+ 1 2))} (try (throw "x") (catch e {:v (quote (+ 1 2))}))))
  (is (= (quote (str "hi")) (try (throw "x") (catch e (quote (str "hi"))))))
  (is (= "boom" (get (try (throw "boom") (catch e {:op (quote (throw "boom")) :error (str e)})) :error)))
  (is (= (quote (throw "boom")) (get (try (throw "boom") (catch e {:op (quote (throw "boom")) :error (str e)})) :op))))

(deftest recur-from-catch-tail
  (is (= 3 (loop [i 0] (try (if (< i 3) (throw "again") i) (catch e (recur (inc i))))))))
