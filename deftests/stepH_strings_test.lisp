;; Replica of tests/stepH_strings.mal: split, subs, starts-with?, ends-with?.

(deftest string-split
  (are [expected expr] (= expected expr)
    ["abc" "abc"]                 (split "abc-abc" "-")
    ["abc-abc"]                   (split "abc-abc" " ")
    ["abc" "abc" "abc" "0" "s"]   (split "abc-abc-abc-0-s" "-")
    ["a" "b" "c" "-" "a" "b" "c"] (split "abc-abc" "")
    [""]                          (split "" "-")
    ["abc abc" "abc"]             (split "abc abc-abc" "-")
    ["abc" "abc-abc"]             (split "abc abc-abc" " ")
    ["" "c" "c-" "c"]             (split "abcabc-abc" "ab"))
  ;; compatible with first
  (is (= "abc" (first (split "abc-abc" "-")))))

(deftest string-subs
  ;; counted in Unicode code points
  (are [expected expr] (= expected expr)
    "el"  (subs "hello" 1 3)
    "llo" (subs "hello" 2)
    "él"  (subs "héllo" 1 3)
    ""    (subs "hello" 0 0)
    ""    (subs "hello" 5)))

(deftest string-prefix-suffix
  (is (starts-with? "--foo" "--"))
  (is (= false (starts-with? "-f" "--")))
  (is (ends-with? "file.txt" ".txt"))
  (is (= false (ends-with? "file.txt" ".md"))))
