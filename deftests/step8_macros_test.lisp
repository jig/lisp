;; Replica of tests/step8_macros.mal: macro definition, macroexpand,
;; cond, and nth/first/rest.

(deftest macros-trivial
  (defmacro macros-one (fn () 1))
  (is (= 1 (macros-one)))
  (defmacro macros-two (fn () 2))
  (is (= 2 (macros-two))))

(deftest macros-unless
  (defmacro macros-unless (fn (pred a b) `(if ~pred ~b ~a)))
  (is (= 7 (macros-unless false 7 8)))
  (is (= 8 (macros-unless true 7 8)))
  (defmacro macros-unless2 (fn (pred a b) (list (quote if) (list (quote not) pred) a b)))
  (is (= 7 (macros-unless2 false 7 8)))
  (is (= 8 (macros-unless2 true 7 8))))

(deftest macros-macroexpand
  (defmacro macros-one2 (fn () 1))
  (is (= 1 (macroexpand (macros-one2))))
  (defmacro macros-unless3 (fn (pred a b) `(if ~pred ~b ~a)))
  (is (= (quote (if PRED B A)) (macroexpand (macros-unless3 PRED A B))))
  (defmacro macros-unless4 (fn (pred a b) (list (quote if) (list (quote not) pred) a b)))
  (is (= (quote (if (not PRED) A B)) (macroexpand (macros-unless4 PRED A B))))
  (is (= (quote (if (not 2) 3 4)) (macroexpand (macros-unless4 2 3 4)))))

(deftest macros-result-is-evaluated
  (defmacro macros-identity (fn (x) x))
  (is (= (quote a) (let (a 123) (macroexpand (macros-identity a)))))
  (is (= 123 (let (a 123) (macros-identity a)))))

(deftest macros-do-not-break-basics
  (is (= () ()))
  (is (list? ()))
  (is (= (quote (1)) `(1))))

;; -------- Deferrable Functionality --------

(deftest not-is-a-function
  (is (= false (not (= 1 1))))
  (is (= true (not (not= 1 1))))
  (is (= true (not (= 1 2))))
  (is (= false (not (not= 1 2)))))

(deftest nth-first-rest-lists
  (are [expected expr] (= expected expr)
    1   (nth (list 1) 0)
    2   (nth (list 1 2) 1)
    nil (nth (list 1 2 nil) 2)
    nil (first (list))
    6   (first (list 6))
    7   (first (list 7 8 9))
    ()  (rest (list))
    ()  (rest (list 6))
    (quote (8 9)) (rest (list 7 8 9)))
  ;; nth out of range throws and aborts the def
  (def macros-x "x")
  (is (= :threw (try (def macros-x (nth (list 1 2) 2)) (catch e :threw))))
  (is (= "x" macros-x)))

(deftest cond-macro
  (is (= nil (macroexpand (cond))))
  (is (= nil (cond)))
  (is (= (quote (if X Y (cond))) (macroexpand (cond X Y))))
  (are [expected expr] (= expected expr)
    7   (cond true 7)
    nil (cond false 7)
    7   (cond true 7 true 8)
    8   (cond false 7 true 8)
    9   (cond false 7 false 8 "else" 9)
    8   (cond false 7 (= 2 2) 8 "else" 9)
    nil (cond false 7 false 8 false 9))
  (is (= (quote (if X Y (cond Z T))) (macroexpand (cond X Y Z T))))
  (is (= "yes" (let (x (cond false "no" true "yes")) x)))
  (is (= "yes" (let [x (cond false "no" true "yes")] x))))

(deftest nth-first-rest-vectors
  (are [expected expr] (= expected expr)
    1   (nth [1] 0)
    2   (nth [1 2] 1)
    nil (nth [1 2 nil] 2)
    nil (first [])
    nil (first nil)
    10  (first [10])
    10  (first [10 11 12])
    ()  (rest [])
    ()  (rest nil)
    ()  (rest [10])
    (quote (11 12)) (rest [10 11 12])
    (quote (11 12)) (rest (cons 10 [11 12])))
  (def macros-x2 "x")
  (is (= :threw (try (def macros-x2 (nth [1 2] 2)) (catch e :threw))))
  (is (= "x" macros-x2)))

;; ------- Optional Functionality --------------

(deftest macros-use-closures
  (def macros-closure-x 2)
  (defmacro macros-a (fn [] macros-closure-x))
  (is (= 2 (macros-a)))
  (is (= 2 (let (macros-closure-x 3) (macros-a)))))
