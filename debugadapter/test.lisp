;; test.lisp

;; this is a simple code to test the lisp-debugger

(defn sqr
    "Returns the square of a number."
    [x]
    (* x x))

(do
    (def a 33)
    (println 1)
    (println 2)
    (println 3)
    (if (= 1 2)
        (println 4)
        (do
            (def b (sqr a))
            (println b))
    ))

(def a2 {:a "1990" :b "1991" :c "1992"})
(println a2)

;; debugging with threading macro
(def a3 (-> {}
             (assoc :a "1905")
             (assoc :b "1915")
             (assoc :c "1925")))
(println a3)

;; debugging with thread-last macro: the value is inserted at the END
;; of each stage — classic map/reduce pipeline
(def a4 (->> (range 1 6)
             (map sqr)
             (reduce + 0)))
(println a4)

;; debugging with cond macro: expands to nested ifs, F11 should walk
;; each test in turn
(println (cond
    (< a4 10)  "small"
    (< a4 100) "medium"
    :else      "large"))

;; debugging across files: cube is defined in testlib.lisp, resolved by
;; require through the search path (here it finds <git root>/.lisp/
;; regardless of the working directory) — F11 on the (testlib/cube …)
;; call must step into that file. require namespaces the module's
;; definitions.
(require "testlib")
(def a5 (testlib/cube 3))
(println a5)

;; :as gives the module a short alias prefix (the module is already
;; loaded, so this only publishes the alias — no re-evaluation)
(require "testlib" :as "tl")
(println (tl/cube 4))

;; :refer imports the listed names unqualified
(require "testlib" :refer ["cube"])
(println (cube 5))

;; lisp defined functions within "ns<namespace>.Load" show doc on hover
(inc 3)
