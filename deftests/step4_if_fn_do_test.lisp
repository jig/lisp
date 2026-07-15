;; Replica of tests/step4_if_fn_do.mal: lists, if, conditionals,
;; equality, fn/closures, do, recursion, strings, varargs, keywords and
;; vectors. The prn/println stdout sections stay in the legacy harness.

(deftest list-functions
  (is (= () (list)))
  (is (list? (list)))
  (is (empty? (list)))
  (is (= false (empty? (list 1))))
  (is (= (quote (1 2 3)) (list 1 2 3)))
  (is (= 3 (count (list 1 2 3))))
  (is (= 0 (count (list))))
  (is (= 0 (count nil)))
  (is (= 78 (if (> (count (list 1 2 3)) 3) 89 78)))
  (is (= 89 (if (>= (count (list 1 2 3)) 3) 89 78))))

(deftest if-form
  (are [expected expr] (= expected expr)
    7     (if true 7 8)
    8     (if false 7 8)
    false (if false 7 false)
    8     (if true (+ 1 7) (+ 1 8))
    9     (if false (+ 1 7) (+ 1 8))
    8     (if nil 7 8)
    7     (if 0 7 8)
    7     (if (list) 7 8)
    7     (if (list 1 2 3) 7 8))
  (is (= false (= (list) nil)))
  (is (= true (not= (list) nil)))
  ;; 1-way if
  (are [expected expr] (= expected expr)
    nil (if false (+ 1 7))
    nil (if nil 8)
    7   (if nil 8 7)
    8   (if true (+ 1 7))))

(deftest basic-conditionals
  (are [expected expr] (= expected expr)
    false (= 2 1)
    true  (= 1 1)
    false (= 1 2)
    false (= 1 (+ 1 1))
    true  (= 2 (+ 1 1))
    false (= nil 1)
    true  (= nil nil)
    true  (not= 2 1)
    false (not= 1 1)
    true  (not= 1 2)
    true  (not= 1 (+ 1 1))
    false (not= 2 (+ 1 1))
    true  (not= nil 1)
    false (not= nil nil)
    true  (> 2 1)
    false (> 1 1)
    false (> 1 2)
    true  (>= 2 1)
    true  (>= 1 1)
    false (>= 1 2)
    false (< 2 1)
    false (< 1 1)
    true  (< 1 2)
    false (<= 2 1)
    true  (<= 1 1)
    true  (<= 1 2)))

(deftest equality-of-lists
  (are [expected expr] (= expected expr)
    true  (= (list) (list))
    true  (= (list) ())
    true  (= (list 1 2) (list 1 2))
    false (= (list 1) (list))
    false (= (list) (list 1))
    false (= 0 (list))
    false (= (list) 0)
    false (= (list nil) (list))
    false (not= (list) (list))
    true  (not= (list 1) (list))))

(deftest user-defined-functions-and-closures
  (is (= 7 ((fn (a b) (+ b a)) 3 4)))
  (is (= 4 ((fn () 4))))
  (is (= 8 ((fn (f x) (f x)) (fn (a) (+ 1 a)) 7)))
  (is (= 12 (((fn (a) (fn (b) (+ a b))) 5) 7)))
  (def if-gen-plus5 (fn () (fn (b) (+ 5 b))))
  (def if-plus5 (if-gen-plus5))
  (is (= 12 (if-plus5 7)))
  (def if-gen-plusX (fn (x) (fn (b) (+ x b))))
  (def if-plus7 (if-gen-plusX 7))
  (is (= 15 (if-plus7 8))))

(def if-do-a nil)

(deftest do-form
  (is (= 14 (do (def if-do-a 6) 7 (+ if-do-a 8))))
  (is (= nil (do)))
  ;; special forms are case-sensitive: DO is an ordinary symbol
  (def if-DO (fn (a) 7))
  (is (= 7 (if-DO 3))))

(deftest recursive-functions
  (def if-sumdown (fn (N) (if (> N 0) (+ N (if-sumdown (- N 1))) 0)))
  (is (= 1 (if-sumdown 1)))
  (is (= 3 (if-sumdown 2)))
  (is (= 21 (if-sumdown 6)))
  (def if-fib (fn (N) (if (= N 0) 1 (if (= N 1) 1 (+ (if-fib (- N 1)) (if-fib (- N 2)))))))
  (is (= 1 (if-fib 1)))
  (is (= 2 (if-fib 2)))
  (is (= 5 (if-fib 4))))

(deftest recursive-functions-in-let
  (is (= 3 (let (f (fn () x) x 3) (f))))
  (is (= nil (let (cst (fn (n) (if (= n 0) nil (cst (- n 1))))) (cst 1))))
  (is (= 0 (let (f (fn (n) (if (= n 0) 0 (g (- n 1)))) g (fn (n) (f n))) (f 2)))))

;; -------- Deferrable Functionality --------

(deftest string-truthiness-and-equality
  (is (= 7 (if "" 7 8)))
  (are [expected expr] (= expected expr)
    true  (= "" "")
    true  (= "abc" "abc")
    false (= "abc" "")
    false (= "" "abc")
    false (= "abc" "def")
    false (= "abc" "ABC")
    false (= (list) "")
    false (= "" (list))
    false (not= "" "")
    true  (not= "abc" "def")))

(deftest variable-length-arguments
  (are [expected expr] (= expected expr)
    3    ((fn (& more) (count more)) 1 2 3)
    true ((fn (& more) (list? more)) 1 2 3)
    1    ((fn (& more) (count more)) 1)
    0    ((fn (& more) (count more)))
    true ((fn (& more) (list? more)))
    2    ((fn (a & more) (count more)) 1 2 3)
    0    ((fn (a & more) (count more)) 1)
    true ((fn (a & more) (list? more)) 1)))

(deftest not-function
  (are [expected expr] (= expected expr)
    true  (not false)
    true  (not nil)
    false (not true)
    false (not "a")
    false (not 0)))

(deftest string-quoting
  (are [expected expr] (= expected expr)
    ""              ""
    "abc"           "abc"
    "abc  def"      "abc  def"
    "\""            "\""
    "abc\ndef\nghi" "abc\ndef\nghi"
    "abc\\def\\ghi" "abc\\def\\ghi"
    "\\n"           "\\n"))

(deftest pr-str-function
  (are [expected expr] (= expected expr)
    ""                    (pr-str)
    "\"\""                (pr-str "")
    "\"abc\""             (pr-str "abc")
    "\"abc  def\" \"ghi jkl\"" (pr-str "abc  def" "ghi jkl")
    "\"\\\"\""            (pr-str "\"")
    "(1 2 \"abc\" \"\\\"\") \"def\"" (pr-str (list 1 2 "abc" "\"") "def")
    "\"abc\\ndef\\nghi\"" (pr-str "abc\ndef\nghi")
    "\"abc\\\\def\\\\ghi\"" (pr-str "abc\\def\\ghi")
    "()"                  (pr-str (list))))

(deftest str-function
  (are [expected expr] (= expected expr)
    ""              (str)
    ""              (str "")
    "abc"           (str "abc")
    "\""            (str "\"")
    "1abc3"         (str 1 "abc" 3)
    "abc  defghi jkl" (str "abc  def" "ghi jkl")
    "abc\ndef\nghi" (str "abc\ndef\nghi")
    "abc\\def\\ghi" (str "abc\\def\\ghi")
    "(1 2 abc \")def" (str (list 1 2 "abc" "\"") "def")
    "()"            (str (list))))

(deftest keywords-equality
  (are [expected expr] (= expected expr)
    true  (= :abc :abc)
    false (= :abc :def)
    false (= :abc ":abc")
    true  (= (list :abc) (list :abc))
    false (not= :abc :abc)
    true  (not= :abc :def)
    true  (not= :abc ":abc")
    false (not= (list :abc) (list :abc))))

(deftest vector-truthiness-and-printing
  (is (= 7 (if [] 7 8)))
  (are [expected expr] (= expected expr)
    "[1 2 \"abc\" \"\\\"\"] \"def\"" (pr-str [1 2 "abc" "\""] "def")
    "[]"              (pr-str [])
    "[1 2 abc \"]def" (str [1 2 "abc" "\""] "def")
    "[]"              (str [])))

(deftest vector-functions-and-equality
  (are [expected expr] (= expected expr)
    3     (count [1 2 3])
    false (empty? [1 2 3])
    true  (empty? [])
    false (list? [4 5 6])
    true  (= [] (list))
    true  (= [7 8] [7 8])
    true  (= [:abc] [:abc])
    true  (= (list 1 2) [1 2])
    false (= (list 1) [])
    false (= [] [1])
    false (= 0 [])
    false (= [] 0)
    false (= [] "")
    false (= "" [])
    false (not= [] (list))
    true  (not= (list 1) [])))

(deftest vector-parameter-lists
  (is (= 4 ((fn [] 4))))
  (is (= 8 ((fn [f x] (f x)) (fn [a] (+ 1 a)) 7))))

(deftest nested-vector-list-equality
  (is (= [(list)] (list [])))
  (is (= [1 2 (list 3 4 [5 6])] (list 1 2 [3 4 (list 5 6)]))))

(deftest empty-and-count-on-maps
  (is (empty? nil))
  ;; empty? on a string is invalid
  (is (= :threw (try (empty? "hello") (catch e :threw))))
  (is (empty? {}))
  (def if-hm {})
  (is (empty? if-hm))
  (is (= 0 (count if-hm)))
  (def if-hm2 {:a 1})
  (is (= false (empty? if-hm2)))
  (is (= 1 (count if-hm2)))
  (def if-hm3 {:a 1 :b {}})
  (is (= false (empty? if-hm3)))
  (is (= 2 (count if-hm3))))
