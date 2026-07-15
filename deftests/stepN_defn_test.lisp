;; Replica of tests/stepN_defn.mal: defn, when and partial.

(deftest defn-macroexpansion
  (is (= (quote (def defn-name (fn args b))) (macroexpand (defn defn-name args b))))
  ;; the printed form of the resulting function keeps the implicit do
  (is (= "(fn [& s] (do (apply str s)))" (pr-str (defn defn-super-str [& s] (apply str s))))))

(deftest defn-bodies-and-varargs
  (defn defn-f [x] 1 2 x)
  (is (= 3 (defn-f 3)))
  (defn defn-super-str2 [& s] (apply str s))
  (is (= "helloworld" (str "hello" "world")))
  (is (= "helloworld" (defn-super-str2 "hello" "world"))))

(deftest when-macro
  (is (= (quote (if condition (do a b))) (macroexpand (when condition a b))))
  (is (= nil (when false (nth () 0) a)))
  (is (= 2 (when true 3 2))))

(deftest partial-application
  (are [expected expr] (= expected expr)
    3       ((partial +) 1 2)
    3       ((partial + 1) 2)
    3       ((partial + 1 2))
    true    ((partial not) false)
    true    ((partial not false))
    3       ((partial (fn [x y] (+ x y)) 1) 2)
    "1234"  ((partial str 1 2) 3 4)))
