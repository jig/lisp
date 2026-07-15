;; Replica of tests/stepP_metatest.mal: a thrown value is caught as-is.

(deftest throw-string-caught
  (is (= "hello" (try (throw "hello") (catch e e)))))
