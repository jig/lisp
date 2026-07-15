;; Replica of tests/stepC_JSON.mal: range, base64 round-trip, ¬…¬ raw
;; strings and str. The prn/println sections are replicated with
;; with-out-str.

(deftest range-basics
  (are [expected expr] (= expected expr)
    [0]         (range 0 1)
    []          (range 0 0)
    [0 1 2 3]   (range 0 4)
    []          (range 0 -1)
    [-10]       (range -10 -9)
    [10]        (range 10 11)))

(deftest base64-round-trip
  (is (= "Hello World" (binary2str (unbase64 (base64 (str2binary "Hello World"))))))
  (is (= "안녕 하세요" (binary2str (unbase64 (base64 (str2binary "안녕 하세요")))))))

(deftest raw-strings
  (is (= "Hello" ¬Hello¬))
  (is (= "\"Hello\"" ¬"Hello"¬))
  ;; a doubled ¬ escapes a literal ¬
  (is (= "¬" ¬¬¬¬))
  (is (= "¬" "¬")))

(deftest str-with-raw-strings
  (are [expected expr] (= expected expr)
    ""                  (str)
    ""                  (str ¬¬)
    "abc"               (str ¬abc¬)
    "¬"                 (str ¬¬¬¬)
    "¬"                 (str "¬")
    "1abc3"             (str 1 ¬abc¬ 3)
    "abc  defghi jkl"   (str ¬abc  def¬ ¬ghi jkl¬)
    "(1 2 abc \")def"   (str (list 1 2 ¬abc¬ ¬"¬) ¬def¬)
    "(1 2 abc ¬)def"    (str (list 1 2 ¬abc¬ ¬¬¬¬) ¬def¬))
  ;; raw strings keep backslashes verbatim
  (is (= "abc\\ndef\\nghi" (str ¬abc\ndef\nghi¬)))
  (is (= "abc\\\\def\\\\ghi" (str ¬abc\\def\\ghi¬))))

(deftest json-shaped-strings-print-raw
  ;; strings that look like JSON print with ¬…¬, others with quotes
  (is (= "¬{\"hello\"}¬" (pr-str "{\"hello\"}")))
  (is (= "\"{hello}\"" (pr-str "{hello}"))))

(deftest prn-with-raw-strings
  (are [expected expr] (= expected expr)
    "\n"        (with-out-str (prn))
    "\"\"\n"    (with-out-str (prn ¬¬))
    "\"abc\"\n" (with-out-str (prn ¬abc¬))
    "\"abc  def\" \"ghi jkl\"\n" (with-out-str (prn ¬abc  def¬ ¬ghi jkl¬))
    "\"\\\"\"\n" (with-out-str (prn ¬"¬))
    "\"¬\"\n"   (with-out-str (prn ¬¬¬¬))
    "\"¬\"\n"   (with-out-str (prn "¬"))
    "\"abc\\\\ndef\\\\nghi\"\n"         (with-out-str (prn ¬abc\ndef\nghi¬))
    "\"abc\\\\\\\\def\\\\\\\\ghi\"\n"   (with-out-str (prn ¬abc\\def\\ghi¬))
    "(1 2 \"abc\" \"\\\"\") \"def\"\n"  (with-out-str (prn (list 1 2 ¬abc¬ ¬"¬) ¬def¬))
    "(1 2 \"abc\" \"¬\") \"def\"\n"     (with-out-str (prn (list 1 2 ¬abc¬ ¬¬¬¬) ¬def¬))
    "¬{\"hello\"}¬\n"  (with-out-str (prn "{\"hello\"}"))
    "\"{hello}\"\n"    (with-out-str (prn "{hello}"))))

(deftest println-with-raw-strings
  (are [expected expr] (= expected expr)
    "\n"    (with-out-str (println))
    "\n"    (with-out-str (println ¬¬))
    "abc\n" (with-out-str (println ¬abc¬))
    "abc  def ghi jkl\n" (with-out-str (println ¬abc  def¬ ¬ghi jkl¬))
    "¬\n"   (with-out-str (println ¬¬¬¬))
    "abc\\ndef\\nghi\n" (with-out-str (println ¬abc\ndef\nghi¬))))
