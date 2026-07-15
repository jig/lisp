;; Replica of tests/step6_file.mal: read-string, slurp, load-file and
;; atoms. File paths are relative to the repository root, like the
;; originals: run the suite from there (TestDeftests does).

(deftest do-not-broken-by-tco
  (is (= 2 (do (do 1 2)))))

(deftest read-string-basics
  (is (= (quote (1 2 (3 4) nil)) (read-string "(1 2 (3 4) nil)")))
  (is (= nil (read-string "nil")))
  (is (= (quote (+ 2 3)) (read-string "(+ 2 3)")))
  ;; a "…" string may contain a literal newline (Clojure-style)
  (is (= "\n" (read-string "\"\n\"")))
  (is (= 7 (read-string "7 ;; comment")))
  (is (= 5 (eval (read-string "(+ 2 3)")))))

(deftest slurp-file
  (is (= "A line of text\n" (slurp "./tests/test.txt")))
  ;; slurping the same file twice works
  (is (= "A line of text\n" (slurp "./tests/test.txt"))))

(deftest load-file-defs
  (is (= "(fn (a) (do (+ 3 a)))" (pr-str (load-file "./tests/inc.mal"))))
  (is (= 8 (inc1 7)))
  (is (= 9 (inc2 7)))
  (is (= 12 (inc3 9))))

(deftest atoms-basics
  (def file-a (atom 2))
  (is (= "«atom 2»" (pr-str file-a)))
  (is (atom? file-a))
  (is (= false (atom? 1)))
  (is (= 2 (deref file-a)))
  (is (= 3 (reset! file-a 3)))
  (is (= 3 (deref file-a)))
  (def file-inc3 (fn (a) (+ 3 a)))
  (is (= 6 (swap! file-a file-inc3)))
  (is (= 6 (deref file-a)))
  (is (= 6 (swap! file-a (fn (a) a))))
  (is (= 12 (swap! file-a (fn (a) (* 2 a)))))
  (is (= 120 (swap! file-a (fn (a b) (* a b)) 10)))
  (is (= 123 (swap! file-a + 3))))

(deftest atoms-and-closures
  (def file-inc-it (fn (a) (+ 1 a)))
  (def file-atm (atom 7))
  (def file-f (fn () (swap! file-atm file-inc-it)))
  (is (= 8 (file-f)))
  (is (= 9 (file-f)))
  ;; closures retain their own atoms
  (def file-g (let (atm (atom 0)) (fn () (deref atm))))
  (def file-atm2 (atom 1))
  (is (= 0 (file-g))))

;; -------- Deferrable Functionality --------

(deftest load-file-large
  (load-file "./tests/computations.mal")
  (is (= 3 (sumdown 2)))
  (is (= 1 (fib 2))))

(deftest deref-reader-macro
  (def file-atm3 (atom 9))
  (is (= 9 @file-atm3)))

(deftest vector-params-not-broken-by-tco
  (def file-g2 (fn [] 78))
  (is (= 78 (file-g2)))
  (def file-g3 (fn [a] (+ a 78)))
  (is (= 81 (file-g3 3))))

(deftest argv-exists
  (is (list? *ARGV*)))

;; eval evaluates in the root environment, so the reference def must be
;; top-level (a def inside a deftest body binds in the test's local env)
(def file-a2 1)

(deftest eval-defines-in-root-scope
  (is (= 7 (let (b 12) (do (eval (read-string "(def file-aa 7)")) file-aa))))
  ;; eval does not use local environments
  (is (= 1 (let (file-a2 2) (eval (read-string "file-a2"))))))

;; -------- Optional Functionality --------

(deftest load-file-with-comments
  (is (= "(fn (a) (do (+ 5 a)))" (pr-str (load-file "./tests/incB.mal"))))
  (is (= 11 (inc4 7)))
  (is (= 12 (inc5 7)))
  (is (= {"a" 1} (load-file "./tests/incC.mal")))
  (is (= {"a" 1} mymap)))

(deftest read-string-comment-characters
  (are [src] (= 1 (read-string src))
    "1;!"
    "1;\""
    "1;#"
    "1;$"
    "1;%"
    "1;'"
    "1;\\"
    "1;\\\\"
    "1;\\\\\\"
    "1;`"
    "1; &()*+,-./:;<=>?@[]^_{|}~"))
