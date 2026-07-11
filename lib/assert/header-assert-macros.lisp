;; $MODULE header-assert-macros

;; assert macros
(do
    (defmacro assert-true
        (with-meta
            (fn [name expr]
                (list
                    'if (try expr (catch err err))
                        nil
                        {   :failed true
                            :name name
                            :expr (str expr)}))
            {:doc "Test case (for test-suite): passes when expr is truthy and does not throw."}))

    (defmacro assert-false
        (with-meta
            (fn [name expr]
                (list
                    'if (try expr (catch err err))
                        {   :failed true
                            :name name
                            :expr (str expr)}
                        nil))
            {:doc "Test case (for test-suite): passes when expr is falsey."}))

    (defmacro assert-throws
        (with-meta
            (fn [name expr]
                (let [failureError {   :failed true
                                        :name (str name)
                                        :expr (str expr)}]
                `(try
                    (do
                        ~expr
                        ~failureError)
                    (catch err nil))))
            {:doc "Test case (for test-suite): passes when evaluating expr raises an error."}))

    (defn test-suite
        "Runs assert-* cases and prints PASS/FAIL for the named suite."
        [name & assert-cases]
        (if
            (reduce and true
                (map
                    (fn [x]
                        (if  (not (nil? x))
                            (println "TEST SUITE FAIL" name ">" (get x :name) ">>" (get x :expr))
                            true))
                    assert-cases))
            (println "TEST SUITE PASS" name "PASS"))))
