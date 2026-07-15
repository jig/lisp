;; Replica of tests/stepG_infunctions.mal: assoc, assoc-in, get-in,
;; update and update-in over maps and vectors.

(deftest assoc-vectors
  (is (= [0 1 100 3] (assoc [0 1 2 3] 2 100)))
  (is (= [0 1 "hello" 3] (assoc [0 1 2 3] 2 "hello"))))

(deftest assoc-in-basics
  (is (= {:a "hello"} (assoc-in {} [:a] "hello")))
  (is (= "hello" (get (assoc-in {:a 1 :b {:c 2}} [:a] "hello") :a)))
  (is (= "hello" (get (assoc-in {:a 1 :b {:c 2}} [:b] "hello") :b)))
  (is (= {:c 2} (get (assoc-in {:a 1 :b {:c 2}} [:a] "hello") :b)))
  (is (= 1 (get (assoc-in {:a 1 :b {:c 2}} [:b] "hello") :a)))
  (is (= [0 1 2] (assoc-in [0 1 2] [] "hello")))
  (is (= [0 1 ["hello"] 3] (assoc-in [0 1 [2] 3] [2 0] "hello")))
  (is (= [0 1 {:a "hello"} 3] (assoc-in [0 1 {:a 10} 3] [2 :a] "hello"))))

(deftest get-in-maps
  (def infn-m {:a 1 :b {:c 2}})
  (are [expected expr] (= expected expr)
    1   (get-in {:a 1} [:a])
    2   (get-in {:a 1 :b {:c 2}} [:b :c])
    nil (get-in {:a 1 :b {:c 2}} [:b :d])
    2   (get-in infn-m [:b :c])
    nil (get-in infn-m [:b :d]))
  (is (= 1 (get (get-in infn-m []) :a)))
  (is (= {:c 2} (get (get-in infn-m []) :b))))

(deftest get-in-vectors
  (are [expected expr] (= expected expr)
    12            (get-in [10 11 12 13] [2])
    201           (get-in [10 11 [200 201 202] 13] [2 1])
    [200 201 202] (get-in [10 11 [200 201 202] 13] [2])
    [10 11 [200 201 202] 13] (get-in [10 11 [200 201 202] 13] [])))

(deftest update-maps-and-vectors
  (def infn-m2 {:a 1 :b {:c 2}})
  (is (= 22 (get (update infn-m2 :a (fn [_] 22)) :a)))
  (is (= 33 (get (update infn-m2 :x (fn [_] 33)) :x)))
  (def infn-v [0 1 [22 33] 3])
  (is (= 1111 (get (update infn-v 1 (fn [_] 1111)) 1)))
  (is (= 5555 (get (update infn-v 2 (fn [_] 5555)) 2))))

(deftest update-in-maps
  (def infn-m3 {:a 1 :b {:c 2}})
  (def infn-mu (update-in infn-m3 [:b :c] (fn [x] (+ 1000 x))))
  (is (= 1002 (get-in infn-mu [:b :c])))
  (is (= 1 (get-in infn-mu [:a])))
  (is (= {:c 1002} (get-in infn-mu [:b])))
  ;; an empty path applies the fn to nil and leaves the map unchanged
  (is (= 2 (get-in (update-in infn-m3 [] (fn [x] (+ x 2000))) [:b :c])))
  ;; the fn receives nil for missing paths
  (is (= 200 (get-in (update-in infn-m3 [:x] (fn [x] (if x 100 200))) [:x])))
  (is (= 300 (get-in (update-in infn-m3 [:x :y] (fn [x] (if x 100 300))) [:x :y])))
  (is (= 400 (get-in (update-in infn-m3 [:x :y :z] (fn [x] (if x 100 400))) [:x :y :z]))))

(deftest update-in-vectors
  (def infn-v2 [0 1 [22 33] 3])
  (def infn-vu2 (update-in infn-v2 [2 1] (fn [x] (+ 1000 x))))
  (is (= [22 1033] (get infn-vu2 2)))
  (is (= 1033 (get-in infn-vu2 [2 1])))
  (is (= 0 (get-in infn-vu2 [0]))))

(deftest assoc-in-positions
  (def infn-m4 {:a 1 :b {:c 2}})
  (are [expected path value] (= expected (get-in (assoc-in infn-m4 path value) path))
    11 [:b :c]    11
    12 [:b]       12
    13 [:a]       13
    14 [:x]       14
    18 [:x :y]    18
    19 [:x :y :z] 19))

(deftest get-in-mixed-index
  (is (= 20 (get-in {:a [10 20]} [:a 1])))
  (is (= 20 (get-in {:a (quote (10 20))} [:a 1])))
  (is (= "hello" (get-in {:a (quote (10 [30 40 {:b "hello"} 60]))} [:a 1 2 :b]))))
