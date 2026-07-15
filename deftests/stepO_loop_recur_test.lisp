;; Replica of tests/stepO_loop_recur.mal: loop/recur, fn-recur and
;; their errors. The issue-#87 catch-result cases live in
;; step9_try_test.lisp.

(deftest loop-without-recur-is-let
  (are [expected expr] (= expected expr)
    7  (loop [] 7)
    11 (loop [a 11] a)
    7  (loop [a 3 b 4] (+ a b))
    ;; bindings evaluate sequentially, like let
    8  (loop [a 2 b (* a 3)] (+ a b))
    ;; list-style binding form is also accepted
    30 (loop (a 5 b 6) (* a b))
    ;; the loop body is an implicit do
    3  (loop [i 0] 1 2 3)))

(def loop-shadowed 99)

(deftest loop-bindings-shadow
  (is (= 1 (loop [loop-shadowed 1] loop-shadowed)))
  (is (= 99 loop-shadowed))
  (is (= 2 (let [x 1] (loop [x 2] x)))))

(deftest recur-basics
  ;; countdown and accumulation
  (is (= 15 (loop [i 5 acc 0] (if (= i 0) acc (recur (- i 1) (+ acc i))))))
  ;; recur rebinds all bindings positionally
  (is (= 13 (loop [a 0 b 10] (if (= a 3) b (recur (inc a) (inc b))))))
  ;; recur is TCO: deep iteration runs in constant stack
  (is (= 100000 (loop [i 0] (if (< i 100000) (recur (inc i)) i)))))

(deftest recur-nested-loops
  ;; recur binds to the nearest enclosing loop
  (is (= 6 (loop [i 0 total 0]
             (if (= i 3)
               total
               (recur (inc i) (+ total (loop [j 0 s 0] (if (= j 2) s (recur (inc j) (+ s 1))))))))))
  ;; a loop may compute another loop's initial binding
  (is (= [0 1 2] (loop [v (loop [j 0 acc []] (if (< j 3) (recur (inc j) (conj acc j)) acc))] v))))

(deftest loop-inside-function
  (defn loop-sum-to [n] (loop [i 0 acc 0] (if (> i n) acc (recur (inc i) (+ acc i)))))
  (is (= 10 (loop-sum-to 4)))
  (is (= 5050 (loop-sum-to 100))))

(deftest recur-errors
  ;; recur arity must match the loop bindings
  (is (= :threw (try (loop [i 0] (if (< i 1) (recur 1 2) i)) (catch e :threw))))
  ;; recur outside any loop or function is an error
  (is (= :threw (try (eval (read-string "(recur 1)")) (catch e :threw)))))

(deftest fn-recur
  (defn loop-count-up [x] (if (> x 3) x (recur (inc x))))
  (is (= 4 (loop-count-up 0)))
  (is (= 10 (loop-count-up 10)))
  ;; anonymous function
  (is (= 15 ((fn [i acc] (if (= i 0) acc (recur (- i 1) (+ acc i)))) 5 0)))
  ;; constant stack
  (defn loop-spin [i] (if (< i 100000) (recur (inc i)) i))
  (is (= 100000 (loop-spin 0)))
  ;; a tail call moves the recursion point to the called function
  (defn loop-pong [x] (if (> x 2) :pong-done (recur (+ x 1))))
  (defn loop-ping [x] (loop-pong x))
  (is (= :pong-done (loop-ping 0))))

(deftest fn-recur-nearest-point
  ;; a loop inside a fn wins as the nearest recursion point
  (defn loop-sum-below [n] (loop [i 0 acc 0] (if (< i n) (recur (inc i) (+ acc i)) acc)))
  (is (= 6 (loop-sum-below 4)))
  ;; a function called from a loop body recurs to itself
  (defn loop-count-up2 [x] (if (> x 3) x (recur (inc x))))
  (is (= 4 (loop [i 0] (if (< i 2) (recur (inc i)) (loop-count-up2 0))))))

(deftest fn-recur-through-go-paths
  (defn loop-count-up3 [x] (if (> x 3) x (recur (inc x))))
  (is (= 4 (apply loop-count-up3 [0])))
  (is (= (quote (4 10)) (map loop-count-up3 [0 10])))
  (def loop-counter (atom 0))
  (is (= 5 (swap! loop-counter (fn [v] (if (< v 5) (recur (inc v)) v))))))

(deftest fn-recur-from-try-tail
  (defn loop-retry [x] (try (if (< x 3) (recur (inc x)) x) (catch e :nope)))
  (is (= 3 (loop-retry 0))))

(deftest fn-recur-arity
  (defn loop-bad-arity [x] (if false x (recur 1 2)))
  (is (= :threw (try (loop-bad-arity 0) (catch e :threw)))))
