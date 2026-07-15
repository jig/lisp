;; Replica of tests/step7_quote.mal: cons, concat, quote, quasiquote,
;; unquote, splice-unquote, vec and quasiquoteexpand.

(deftest cons-function
  (are [expected expr] (= expected expr)
    (quote (1))       (cons 1 (list))
    (quote (1 2))     (cons 1 (list 2))
    (quote (1 2 3))   (cons 1 (list 2 3))
    (quote ((1) 2 3)) (cons (list 1) (list 2 3)))
  (def quote-a1 (list 2 3))
  (is (= (quote (1 2 3)) (cons 1 quote-a1)))
  ;; cons does not mutate its argument
  (is (= (quote (2 3)) quote-a1)))

(deftest concat-function
  (are [expected expr] (= expected expr)
    ()                    (concat)
    (quote (1 2))         (concat (list 1 2))
    (quote (1 2 3 4))     (concat (list 1 2) (list 3 4))
    (quote (1 2 3 4 5 6)) (concat (list 1 2) (list 3 4) (list 5 6))
    ()                    (concat (concat))
    ()                    (concat (list) (list)))
  (is (= () (concat)))
  (is (= false (not= () (concat))))
  (def quote-a2 (list 1 2))
  (def quote-b2 (list 3 4))
  (is (= (quote (1 2 3 4 5 6)) (concat quote-a2 quote-b2 (list 5 6))))
  ;; concat does not mutate its arguments
  (is (= (quote (1 2)) quote-a2))
  (is (= (quote (3 4)) quote-b2)))

(deftest quote-basics
  (is (= 7 (quote 7)))
  (is (= (list 1 2 3) (quote (1 2 3))))
  (is (= (list 1 2 (list 3 4)) (quote (1 2 (3 4))))))

(deftest quasiquote-simple
  (are [expected expr] (= expected expr)
    nil (quasiquote nil)
    7   (quasiquote 7))
  (is (= (quote a) (quasiquote a)))
  (is (= (quote {"a" b}) (quasiquote {"a" b}))))

(deftest quasiquote-lists
  (are [expected expr] (= expected expr)
    ()                     (quasiquote ())
    (quote (1 2 3))        (quasiquote (1 2 3))
    (quote (a))            (quasiquote (a))
    (quote (1 2 (3 4)))    (quasiquote (1 2 (3 4)))
    (quote (nil))          (quasiquote (nil))
    (quote (1 ()))         (quasiquote (1 ()))
    (quote (() 1))         (quasiquote (() 1))
    (quote (1 () 2))       (quasiquote (1 () 2))
    (quote (()))           (quasiquote (()))))

(def quote-ua 8)
(def quote-ub (quote (1 "b" "d")))

(deftest unquote-basics
  (is (= 7 (quasiquote (unquote 7))))
  (is (= (quote quote-ua) (quasiquote quote-ua)))
  (is (= 8 (quasiquote (unquote quote-ua))))
  (is (= (quote (1 quote-ua 3)) (quasiquote (1 quote-ua 3))))
  (is (= (quote (1 8 3)) (quasiquote (1 (unquote quote-ua) 3))))
  (is (= (quote (1 quote-ub 3)) (quasiquote (1 quote-ub 3))))
  (is (= (quote (1 (1 "b" "d") 3)) (quasiquote (1 (unquote quote-ub) 3))))
  (is (= (quote (1 2)) (quasiquote ((unquote 1) (unquote 2)))))
  ;; quasiquote and environments
  (is (= 0 (let (x 0) (quasiquote (unquote x))))))

(def quote-uc (quote (1 "b" "d")))

(deftest splice-unquote-basics
  (is (= (quote (1 quote-uc 3)) (quasiquote (1 quote-uc 3))))
  (is (= (quote (1 1 "b" "d" 3)) (quasiquote (1 (splice-unquote quote-uc) 3))))
  (is (= (quote (1 1 "b" "d")) (quasiquote (1 (splice-unquote quote-uc)))))
  (is (= (quote (1 "b" "d" 2)) (quasiquote ((splice-unquote quote-uc) 2))))
  (is (= (quote (1 "b" "d" 1 "b" "d")) (quasiquote ((splice-unquote quote-uc) (splice-unquote quote-uc))))))

(deftest symbol-equality
  (are [expected expr] (= expected expr)
    true  (= (quote abc) (quote abc))
    false (= (quote abc) (quote abcd))
    false (= (quote abc) "abc")
    false (= "abc" (quote abc))
    true  (= "abc" (str (quote abc)))
    false (= (quote abc) nil)
    false (= nil (quote abc))))

;; -------- Deferrable Functionality --------

(deftest quote-reader-macro
  (is (= 7 '7))
  (is (= (list 1 2 3) '(1 2 3)))
  (is (= (list 1 2 (list 3 4)) '(1 2 (3 4)))))

(deftest cons-concat-with-vectors
  (are [expected expr] (= expected expr)
    (quote (1))       (cons 1 [])
    (quote ([1] 2 3)) (cons [1] [2 3])
    (quote (1 2 3))   (cons 1 [2 3])
    (quote (1 2 3 4 5 6)) (concat [1 2] (list 3 4) [5 6])
    (quote (1 2))     (concat [1 2])))

;; -------- Optional Functionality --------

(deftest quasiquote-reader-macros
  (is (= 7 `7))
  (is (= (quote (1 2 3)) `(1 2 3)))
  (is (= (quote (1 2 (3 4))) `(1 2 (3 4))))
  (is (= (quote (nil)) `(nil)))
  (is (= 7 `~7))
  (is (= (quote (1 8 3)) `(1 ~quote-ua 3)))
  (is (= (quote (1 quote-ub 3)) `(1 quote-ub 3)))
  (is (= (quote (1 (1 "b" "d") 3)) `(1 ~quote-ub 3)))
  (is (= (quote (1 quote-uc 3)) `(1 quote-uc 3)))
  (is (= (quote (1 1 "b" "d" 3)) `(1 ~@quote-uc 3))))

(deftest vec-function
  (are [expected expr] (= expected expr)
    []    (vec (list))
    [1]   (vec (list 1))
    [1 2] (vec (list 1 2))
    []    (vec [])
    [1 2] (vec [1 2]))
  ;; vec does not mutate the original list
  (def quote-va (list 1 2))
  (is (= [1 2] (vec quote-va)))
  (is (= (quote (1 2)) quote-va)))

(deftest quine
  (def quote-quine (quote ((fn (q) (quasiquote ((unquote q) (quote (unquote q))))) (quote (fn (q) (quasiquote ((unquote q) (quote (unquote q)))))))))
  (is (= quote-quine (eval quote-quine))))

(deftest quasiquote-with-vectors
  (are [expected expr] (= expected expr)
    []   (quasiquote [])
    [[]] (quasiquote [[]])
    [()] (quasiquote [()])
    (quote ([])) (quasiquote ([]))
    (quote [1 a 3]) `[1 a 3]
    (quote [a [] b [c] d [e f] g]) (quasiquote [a [] b [c] d [e f] g])
    [8]           `[~quote-ua]
    (quote [(8)]) `[(~quote-ua)]
    (quote ([8])) `([~quote-ua])
    (quote [a 8 a]) `[a ~quote-ua a]
    (quote ([a 8 a])) `([a ~quote-ua a])
    (quote [(a 8 a)]) `[(a ~quote-ua a)]
    (quote [1 "b" "d"])     `[~@quote-uc]
    (quote [(1 "b" "d")])   `[(~@quote-uc)]
    (quote ([1 "b" "d"]))   `([~@quote-uc])
    (quote [1 1 "b" "d" 3]) `[1 ~@quote-uc 3]
    (quote ([1 1 "b" "d" 3])) `([1 ~@quote-uc 3])
    (quote [(1 1 "b" "d" 3)]) `[(1 ~@quote-uc 3)]))

(deftest misplaced-unquote
  (is (= (quote (0 unquote)) `(0 unquote)))
  (is (= (quote (0 splice-unquote)) `(0 splice-unquote)))
  (is (= (quote [unquote 0]) `[unquote 0]))
  (is (= (quote [splice-unquote 0]) `[splice-unquote 0])))

(deftest quasiquoteexpand-forms
  (are [expected expr] (= expected expr)
    nil (quasiquoteexpand nil)
    7   (quasiquoteexpand 7)
    (quote (quote a))       (quasiquoteexpand a)
    (quote (quote {"a" b})) (quasiquoteexpand {"a" b})
    ()  (quasiquoteexpand ())
    (quote (cons 1 (cons 2 (cons 3 ()))))  (quasiquoteexpand (1 2 3))
    (quote (cons (quote a) ()))            (quasiquoteexpand (a))
    (quote (cons 1 (cons 2 (cons (cons 3 (cons 4 ())) ())))) (quasiquoteexpand (1 2 (3 4)))
    (quote (cons nil ()))                  (quasiquoteexpand (nil))
    (quote (cons 1 (cons () ())))          (quasiquoteexpand (1 ()))
    (quote (cons () (cons 1 ())))          (quasiquoteexpand (() 1))
    (quote (cons 1 (cons () (cons 2 ())))) (quasiquoteexpand (1 () 2))
    (quote (cons () ()))                   (quasiquoteexpand (()))
    7 (quasiquoteexpand (unquote 7))
    (quote a) (quasiquoteexpand (unquote a))
    (quote (cons 1 (cons (quote a) (cons 3 ()))))  (quasiquoteexpand (1 a 3))
    (quote (cons 1 (cons a (cons 3 ()))))          (quasiquoteexpand (1 (unquote a) 3))
    (quote (cons 1 (cons (quote b) (cons 3 ())))) (quasiquoteexpand (1 b 3))
    (quote (cons 1 (cons b (cons 3 ()))))         (quasiquoteexpand (1 (unquote b) 3))
    (quote (cons 1 (cons 2 ())))                  (quasiquoteexpand ((unquote 1) (unquote 2)))
    (quote (cons (quote a) (concat (b c) (cons (quote d) ())))) (quasiquoteexpand (a (splice-unquote (b c)) d))
    (quote (cons 1 (cons (quote c) (cons 3 ())))) (quasiquoteexpand (1 c 3))
    (quote (cons 1 (concat c (cons 3 ()))))       (quasiquoteexpand (1 (splice-unquote c) 3))
    (quote (cons 1 (concat c ())))                (quasiquoteexpand (1 (splice-unquote c)))
    (quote (concat c (cons 2 ())))                (quasiquoteexpand ((splice-unquote c) 2))
    (quote (concat c (concat c ())))              (quasiquoteexpand ((splice-unquote c) (splice-unquote c)))
    (quote (vec ()))                              (quasiquoteexpand [])
    (quote (vec (cons (vec ()) ())))              (quasiquoteexpand [[]])
    (quote (vec (cons () ())))                    (quasiquoteexpand [()])
    (quote (cons (vec ()) ()))                    (quasiquoteexpand ([]))
    (quote (vec (cons 1 (cons (quote a) (cons 3 ()))))) (quasiquoteexpand [1 a 3])))

(deftest vec-on-sets
  (is (= [] (vec #{})))
  (is (= [:a] (vec #{:a})))
  (is (= (set (vec #{:a :b})) #{:a :b}))
  (is (= (set (vec #{:a :b :c :d})) #{:a :b :c :d}))
  (is (= ["a"] (vec #{"a"})))
  (is (= (set (vec #{"a" "b"})) #{"a" "b"}))
  (is (= (set (vec #{"a" "b" "c" "d"})) #{"a" "b" "c" "d"})))
