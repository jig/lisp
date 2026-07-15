;; $MODULE header-test

;; deftest/is/are macros, expanding into the test/* builtins
(do
    (defmacro deftest
        (with-meta
            (fn [name & body]
                `(test/register! (quote ~name) (fn [] ~@body)))
            {:doc "Registers body as the test named name; run with the --test runner or (test/run-tests!)."}))

    (defmacro is
        (with-meta
            (fn [form & msg]
                ;; only core builtins here: coreextended (and, …) may not be loaded
                (let [eq2 (if (list? form)
                              (if (= (first form) (quote =))
                                  (= 3 (count form))
                                  false)
                              false)]
                    (if eq2
                        `(test/check-eq! (quote ~form) (fn [] ~(nth form 1)) (fn [] ~(nth form 2)) ~@msg)
                        `(test/check! (quote ~form) (fn [] ~form) ~@msg))))
            {:doc "Asserts form is truthy; inside deftest it records the outcome, outside it throws on failure. (is (= expected actual)) reports both values."}))

    (defmacro are
        (with-meta
            (fn [argv expr & rows]
                (test/expand-are argv expr rows))
            {:doc "Template assertion: substitutes each row of values for argv in expr and asserts every instance, e.g. (are [x y] (= x y) 2 (+ 1 1) 4 (* 2 2))."}))

    (defmacro with-out-str
        (with-meta
            (fn [& body]
                `(test/with-out-str* (fn [] ~@body)))
            {:doc "Evaluates body capturing standard output and returns it as a string (Clojure-style); output from concurrent goroutines is captured too."})))
