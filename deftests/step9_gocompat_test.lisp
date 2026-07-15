;; Replica of tests/step9_gocompat.mal: go errors, wrapping, panic and
;; type?. The «go-error …» literal expectations become value/pr-str
;; checks (the literals themselves need an environment-aware reader).

(deftest gocompat-catch-values
  (is (= "sample" (try (throw "sample") (catch err err))))
  (is (= "string" (try (throw "sample") (catch err (type? err)))))
  (is (= {:a 1} (try (throw {:a 1}) (catch err err)))))

(deftest gocompat-wrapped-errors
  (def gocompat-thrower (fn [] (throw (go-error "wrapped %w" (go-error "sample")))))
  (is (= :threw (try (gocompat-thrower) (catch err :threw))))
  (is (= "«go-error \"wrapped sample\"»" (pr-str (try (gocompat-thrower) (catch err err)))))
  (is (= "«go-error \"wrapped sample\"»" (try (gocompat-thrower) (catch err (str err)))))
  (is (= "go-error" (type? (try (gocompat-thrower) (catch err err)))))
  (is (= :threw (try (throw 9) (catch err :threw)))))

(deftest gocompat-go-error-construction
  (is (= "«go-error \"simple\"»" (pr-str (go-error "simple"))))
  (is (= "«go-error \"simple wrapped\"»" (pr-str (go-error "simple %s" (go-error "wrapped")))))
  (is (= "«go-error \"simple wrapped\"»" (pr-str (go-error "simple %w" (go-error "wrapped")))))
  ;; %w wraps (unwrap-error recovers the cause), %s does not
  (def gocompat-compo-err (go-error "simple %w" (go-error "wrapped")))
  (is (= "«go-error \"wrapped\"»" (pr-str (unwrap-error gocompat-compo-err))))
  (def gocompat-non-compo-err (go-error "simple %s" (go-error "wrapped")))
  (is (= nil (unwrap-error gocompat-non-compo-err))))

(deftest gocompat-throw-go-errors
  (def gocompat-compo-err2 (go-error "simple %w" (go-error "wrapped")))
  (is (= "«go-error \"simple wrapped\"»" (pr-str (try (throw gocompat-compo-err2) (catch err err)))))
  (is (= "«go-error \"simple wrapped\"»" (try (throw gocompat-compo-err2) (catch err (str err)))))
  (is (= "«go-error \"wrapped\"»" (pr-str (try (throw gocompat-compo-err2) (catch err (unwrap-error err)))))))

(deftest gocompat-panic
  (is (= "simple" (try (panic "simple") (catch e e))))
  (is (= "«go-error \"github.com/jig/lisp/lib/core[panic]: simple\"»" (pr-str (try (panic (go-error "simple")) (catch e e)))))
  (is (= "«go-error \"simple\"»" (pr-str (unwrap-error (try (panic (go-error "simple")) (catch e e))))))
  (is (= 3 (try (panic 3) (catch e e)))))

(deftest gocompat-type-predicate
  (are [expected expr] (= expected expr)
    "nil"         (type? nil)
    "boolean"     (type? false)
    "keyword"     (type? :idx)
    "integer"     (type? 3)
    "string"      (type? "hello 世界!")
    "atom"        (type? (atom 3))
    "future-call" (type? (future 3))
    "integer"     (type? (try (throw 3) (catch e e)))
    "list"        (type? (quote (1 2 3)))
    "hash-map"    (type? {:a 1 :b 2 :c 3})
    "vector"      (type? [0 1 :c []])
    "set"         (type? #{:a :b})
    "integer"     (type? (try (panic 3) (catch e e)))
    "go-error"    (type? (go-error "pum"))
    "go-error"    (type? (go-error "pum %s" (go-error "pum!")))
    "go-error"    (type? (go-error "pum %w" (go-error "pum!")))
    "function"    (type? (fn [] 0))
    "go-function" (type? type?)
    "string"      (type? (try (panic "simple") (catch err err))))
  (def gocompat-zero 0)
  (is (= "symbol" (type? (quote gocompat-zero)))))
