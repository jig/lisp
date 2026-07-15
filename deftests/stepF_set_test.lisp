;; Replica of tests/stepF_set.mal: set literals, assoc/dissoc/get on
;; sets, equality, metadata and hash-set.

(deftest sets-are-not-sequential
  (is (= false (sequential? #{})))
  (is (= false (sequential? #{:a :b :c}))))

(deftest set-construction
  (are [expected expr] (= expected expr)
    #{"a"} (set ["a"])
    #{"a"} (set (quote ("a")))
    #{}    (set nil)
    #{"a"} #{"a"}
    #{"a"} (assoc #{} "a"))
  (is (= "a" (get (assoc (assoc #{"a"} "b") "c") "a"))))

(deftest set-predicates-and-get
  (def set-s1 (set (quote ())))
  (is (set? #{}))
  (is (set? set-s1))
  (is (= false (set? 1)))
  (is (= false (set? "abc")))
  (is (= nil (get nil "a")))
  (is (= nil (get set-s1 "a")))
  (is (= false (contains? set-s1 "a")))
  (def set-s2 (assoc set-s1 "a"))
  ;; the original set is unchanged
  (is (= nil (get set-s1 "a")))
  (is (= false (contains? set-s1 "a")))
  (is (= "a" (get set-s2 "a")))
  (is (contains? set-s2 "a")))

(deftest set-seq-and-count
  (def set-s1b (set ()))
  (def set-s2b (assoc set-s1b "a"))
  (is (= () (seq set-s1b)))
  (is (= (quote ("a")) (seq set-s2b)))
  (is (= (quote ("1")) (seq #{"1"})))
  (is (= 3 (count (seq (assoc set-s2b "b" "c")))))
  (is (= 3 (count (assoc set-s2b "b" "c")))))

(deftest set-keyword-keys
  (is (= :abc (get #{:abc} :abc)))
  (is (contains? #{:abc} :abc))
  (is (= false (contains? #{:abcd} :abc)))
  (is (= #{:bcd} (assoc #{} :bcd)))
  (is (keyword? (nth (seq #{:abc :def}) 0))))

(deftest set-assoc-updates
  (def set-s4 (assoc #{:a :b} :a :c))
  (are [expected key] (= expected (get set-s4 key))
    :a  :a
    :b  :b
    :c  :c
    nil :d))

(deftest set-str-and-pr-str
  (is (= "A#{:abc}Z" (str "A" #{:abc} "Z")))
  (is (= "true.false.nil.:keyw.symb" (str true "." false "." nil "." :keyw "." (quote symb))))
  (is (= "\"A\" #{:abc} \"Z\"" (pr-str "A" #{:abc} "Z")))
  (is (= "true \".\" false \".\" nil \".\" :keyw \".\" symb" (pr-str true "." false "." nil "." :keyw "." (quote symb))))
  ;; multi-element set print order is unstable: accept either
  (def set-s (str #{:abc :def}))
  (is (or (= set-s "#{:abc :def}") (= set-s "#{:def :abc}")))
  (def set-p (pr-str #{:abc :def}))
  (is (or (= set-p "#{:abc :def}") (= set-p "#{:def :abc}"))))

(deftest set-dissoc
  (def set-s3 (assoc (assoc #{} "a") "b"))
  (is (= 2 (count (seq set-s3))))
  (is (= 2 (count set-s3)))
  (is (= #{"b"} (dissoc set-s3 "a")))
  (is (= #{} (dissoc set-s3 "a" "b")))
  (is (= #{} (dissoc set-s3 "a" "b" "c")))
  ;; the original set is unchanged
  (is (= 2 (count (seq set-s3))))
  (is (= 2 (count set-s3)))
  (is (= #{:fgh} (dissoc #{:cde :fgh} :cde))))

(deftest set-empty-predicate
  (is (empty? #{}))
  (is (= false (empty? #{"aa"}))))

(deftest set-equality
  (are [expected expr] (= expected expr)
    true  (= #{} #{})
    true  (= #{} (set (quote ())))
    true  (= #{} (set []))
    true  (= #{:a :b} (set [:b :a]))
    true  (= #{:a :b} (set [:a :b]))
    false (= #{:a :c} (set [:a :b]))
    false (= #{:b :c} (set [:a :b]))
    true  (= #{:b :a} (set [:a :b]))
    false (= #{:b} (set [:a]))
    true  (= #{:a :b "c" "d"} (set [:a "c" :b "d"]))
    false (= #{:a :b} (set [:a]))
    false (= #{} [])
    false (= [] #{})
    false (= #{} ())
    false (= () #{})
    false (= #{} {})
    false (= {} #{})))

(deftest set-metadata
  (is (= #{"a"} (meta (with-meta #{"abc"} #{"a"}))))
  (is (set? (with-meta #{"abc"} #{"a"})))
  (is (= #{} (with-meta #{} #{"a"})))
  (def set-l-wm (with-meta [4 5 6] #{"b"}))
  (is (= [4 5 6] set-l-wm))
  (is (= #{"b"} (meta set-l-wm)))
  (is (= #{"new_meta"} (meta (with-meta set-l-wm #{"new_meta"}))))
  (is (= #{"b"} (meta set-l-wm))))

(deftest set-metadata-on-builtins
  (is (= nil (meta +)))
  (def set-f-wm3 ^#{"def"} +)
  (is (= #{"def"} (meta set-f-wm3)))
  (is (= nil (meta +))))

(deftest hash-set-construction
  (are [expected key] (= expected (contains? (hash-set :a :b :c) key))
    true  :a
    true  :b
    true  :c
    false :z))
