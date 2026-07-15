;; Replica of tests/stepA_mal.mal: host-language, atoms as envs,
;; metadata, predicates, conj, seq and time-ms.

(deftest host-language
  (is (= false (= "something bogus" *host-language*))))

(deftest hash-map-evaluation-and-atoms
  (def malA-e (atom {"+" +}))
  (swap! malA-e assoc "-" -)
  (is (= 15 ((get @malA-e "+") 7 8)))
  (is (= 3 ((get @malA-e "-") 11 8)))
  (swap! malA-e assoc "foo" (list))
  (is (= () (get @malA-e "foo")))
  (swap! malA-e assoc "bar" (quote (1 2 3)))
  (is (= (quote (1 2 3)) (get @malA-e "bar"))))

(deftest optional-functions-present
  (is (= nil (do (list time-ms string? number? seq conj meta with-meta fn?) nil)))
  (is (= (quote (false false false)) (map symbol? (quote (nil false true))))))

;; -------- Optional Functionality --------

(deftest metadata-on-functions
  (is (= nil (meta (fn (a) a))))
  (is (= {"b" 1} (meta (with-meta (fn (a) a) {"b" 1}))))
  (is (= "abc" (meta (with-meta (fn (a) a) "abc"))))
  (def malA-l-wm (with-meta (fn (a) a) {"b" 2}))
  (is (= {"b" 2} (meta malA-l-wm)))
  (is (= {"new_meta" 123} (meta (with-meta malA-l-wm {"new_meta" 123}))))
  (is (= {"b" 2} (meta malA-l-wm)))
  (def malA-f-wm2 ^{"abc" 1} (fn [a] (+ 1 a)))
  (is (= {"abc" 1} (meta malA-f-wm2)))
  ;; meta of native functions is nil (not an error)
  (is (= nil (meta +))))

(deftest closures-and-metadata-coexist
  (def malA-gen-plusX (fn (x) (with-meta (fn (b) (+ x b)) {"meta" 1})))
  (def malA-plus7 (malA-gen-plusX 7))
  (def malA-plus8 (malA-gen-plusX 8))
  (is (= 15 (malA-plus7 8)))
  (is (= {"meta" 1} (meta malA-plus7)))
  (is (= {"meta" 1} (meta malA-plus8)))
  (is (= {"meta" 2} (meta (with-meta malA-plus7 {"meta" 2}))))
  (is (= {"meta" 1} (meta malA-plus8))))

(deftest string-number-predicates
  (are [expected expr] (= expected expr)
    true  (string? "")
    false (string? (quote abc))
    true  (string? "abc")
    false (string? :abc)
    false (string? (keyword "abc"))
    false (string? 234)
    false (string? nil)
    true  (number? 123)
    true  (number? -1)
    false (number? nil)
    false (number? false)
    false (number? "123")))

(deftest fn-macro-predicates
  (def malA-add1 (fn (x) (+ x 1)))
  (are [expected expr] (= expected expr)
    true  (fn? +)
    true  (fn? malA-add1)
    false (fn? cond)
    false (fn? "+")
    false (fn? :+)
    true  (fn? ^{"ismacro" true} (fn () 0))
    true  (macro? cond)
    false (macro? +)
    false (macro? malA-add1)
    false (macro? "+")
    false (macro? :+)
    false (macro? {})))

(deftest conj-function
  (are [expected expr] (= expected expr)
    (quote (1))       (conj (list) 1)
    (quote (2 1))     (conj (list 1) 2)
    (quote (4 2 3))   (conj (list 2 3) 4)
    (quote (6 5 4 2 3)) (conj (list 2 3) 4 5 6)
    (quote ((2 3) 1)) (conj (list 1) (list 2 3))
    [1]         (conj [] 1)
    [1 2]       (conj [1] 2)
    [2 3 4]     (conj [2 3] 4)
    [2 3 4 5 6] (conj [2 3] 4 5 6)
    [1 [2 3]]   (conj [1] [2 3])
    {}          (conj {})
    nil         (conj)
    {:a 1}      (conj {} :a 1))
  (are [expected key] (= expected (get (conj {} :a 1 :b 2) key))
    1   :a
    2   :b
    nil :c)
  (are [expected key] (= expected (get (conj {:a 1} :b 2) key))
    1   :a
    2   :b
    nil :c)
  (is (= #{} (conj #{})))
  (is (= #{:a} (conj #{} :a)))
  (are [expected key] (= expected (get (conj #{} :a :b) key))
    :a  :a
    :b  :b
    nil :c)
  (are [expected key] (= expected (get (conj #{:a} :b) key))
    :a  :a
    :b  :b
    nil :c))

(deftest seq-function
  (are [expected expr] (= expected expr)
    (quote ("a" "b" "c")) (seq "abc")
    (quote (2 3 4))       (seq (quote (2 3 4)))
    (quote (2 3 4))       (seq [2 3 4])
    nil (seq "")
    nil (seq (quote ()))
    nil (seq [])
    nil (seq nil))
  (is (= "this is a test" (apply str (seq "this is a test")))))

(deftest metadata-on-collections
  (is (= nil (meta [1 2 3])))
  (is (= [1 2 3] (with-meta [1 2 3] {"a" 1})))
  (is (= {"a" 1} (meta (with-meta [1 2 3] {"a" 1}))))
  (is (vector? (with-meta [1 2 3] {"a" 1})))
  (is (= "abc" (meta (with-meta [1 2 3] "abc"))))
  (is (= [] (with-meta [] "abc")))
  (is (= {"a" 1} (meta (with-meta (list 1 2 3) {"a" 1}))))
  (is (list? (with-meta (list 1 2 3) {"a" 1})))
  (is (= () (with-meta (list) {"a" 1})))
  (is (empty? (with-meta (list) {"a" 1})))
  (is (= {"a" 1} (meta (with-meta {"abc" 123} {"a" 1}))))
  (is (map? (with-meta {"abc" 123} {"a" 1})))
  (is (= {} (with-meta {} {"a" 1})))
  (def malA-l-wm2 (with-meta [4 5 6] {"b" 2}))
  (is (= [4 5 6] malA-l-wm2))
  (is (= {"b" 2} (meta malA-l-wm2)))
  (is (= {"new_meta" 123} (meta (with-meta malA-l-wm2 {"new_meta" 123}))))
  (is (= {"b" 2} (meta malA-l-wm2))))

(deftest metadata-on-builtins
  (is (= nil (meta +)))
  (def malA-f-wm3 ^{"def" 2} +)
  (is (= {"def" 2} (meta malA-f-wm3)))
  (is (= nil (meta +))))

(deftest time-ms-advances
  (load-file "./tests/computations.mal")
  (def malA-start-time (time-ms))
  (is (= false (= malA-start-time 0)))
  (is (= 500500 (sumdown 1000)))
  (is (> (time-ms) malA-start-time)))

(deftest defmacro-does-not-mutate-function
  (def malA-f (fn [x] (number? x)))
  (defmacro malA-m malA-f)
  (is (= true (malA-f (+ 1 1))))
  (is (= false (malA-m (+ 1 1)))))
