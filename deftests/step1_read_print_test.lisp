;; Replica of tests/step1_read_print.mal — read/print round trips via
;; (pr-str (read-string src)). Note this file gives NEW coverage: the
;; original is skipped by the Go .mal harness.

(deftest read-numbers
  (are [printed src] (= printed (pr-str (read-string src)))
    "1"    "1"
    "7"    "7"
    "7"    "  7   "
    "-123" "-123"))

(deftest read-symbols
  (are [printed src] (= printed (pr-str (read-string src)))
    "+"       "+"
    "abc"     "abc"
    "abc"     "   abc   "
    "abc5"    "abc5"
    "abc-def" "abc-def"
    ;; non-numbers starting with a dash
    "-"       "-"
    "-abc"    "-abc"
    "->>"     "->>"))

(deftest read-lists
  (are [printed src] (= printed (pr-str (read-string src)))
    "(+ 1 2)"       "(+ 1 2)"
    "()"            "()"
    "()"            "( )"
    "(nil)"         "(nil)"
    "((3 4))"       "((3 4))"
    "(+ 1 (+ 2 3))" "(+ 1 (+ 2 3))"
    "(+ 1 (+ 2 3))" "  ( +   1   (+   2 3   )   )  "
    "(* 1 2)"       "(* 1 2)"
    "(** 1 2)"      "(** 1 2)"
    "(* -3 6)"      "(* -3 6)"
    "(() ())"       "(()())"))

;; Commas are whitespace, as in kanaka/mal and Clojure
(deftest read-commas-as-whitespace
  (is (= "(1 2 3)" (pr-str (read-string "(1 2, 3,,,,)"))))
  (is (= [1 2 3] [1, 2, 3])))

;; -------- Deferrable Functionality --------

(deftest read-nil-true-false
  (are [printed src] (= printed (pr-str (read-string src)))
    "nil"   "nil"
    "true"  "true"
    "false" "false"))

(deftest read-strings
  (are [printed src] (= printed (pr-str (read-string src)))
    "\"abc\""               "\"abc\""
    "\"abc\""               "   \"abc\"   "
    "\"abc (with parens)\"" "\"abc (with parens)\""
    "\"abc\\\"def\""        "\"abc\\\"def\""
    "\"\""                  "\"\""
    "\"\\\\\""              "\"\\\\\""
    "\"&\"" "\"&\""
    "\"'\"" "\"'\""
    "\"(\"" "\"(\""
    "\")\"" "\")\""
    "\"*\"" "\"*\""
    "\"+\"" "\"+\""
    "\",\"" "\",\""
    "\"-\"" "\"-\""
    "\"/\"" "\"/\""
    "\":\"" "\":\""
    "\";\"" "\";\""
    "\"<\"" "\"<\""
    "\"=\"" "\"=\""
    "\">\"" "\">\""
    "\"?\"" "\"?\""
    "\"@\"" "\"@\""
    "\"[\"" "\"[\""
    "\"]\"" "\"]\""
    "\"^\"" "\"^\""
    "\"_\"" "\"_\""
    "\"`\"" "\"`\""
    "\"{\"" "\"{\""
    "\"#{\"" "\"#{\""
    "\"}\"" "\"}\""
    "\"~\"" "\"~\""
    "\"!\"" "\"!\""))

(deftest read-errors
  (are [src] (= :threw (try (read-string src) (catch e :threw)))
    "(1 2"
    "[1 2"
    "\"abc"
    "\""
    "\"\\\""
    "(1 \"abc"
    "(1 \"abc\""))

(deftest read-quoting-forms
  (are [printed src] (= printed (pr-str (read-string src)))
    "(quote 1)"                    "'1"
    "(quote (1 2 3))"              "'(1 2 3)"
    "(quasiquote 1)"               "`1"
    "(quasiquote (1 2 3))"         "`(1 2 3)"
    "(unquote 1)"                  "~1"
    "(unquote (1 2 3))"            "~(1 2 3)"
    "(quasiquote (1 (unquote a) 3))" "`(1 ~a 3)"
    "(splice-unquote (1 2 3))"     "~@(1 2 3)"))

(deftest read-keywords
  (are [printed src] (= printed (pr-str (read-string src)))
    ":kw"              ":kw"
    "(:kw1 :kw2 :kw3)" "(:kw1 :kw2 :kw3)"))

(deftest read-vectors
  (are [printed src] (= printed (pr-str (read-string src)))
    "[+ 1 2]"       "[+ 1 2]"
    "[]"            "[]"
    "[]"            "[ ]"
    "[[3 4]]"       "[[3 4]]"
    "[+ 1 [+ 2 3]]" "[+ 1 [+ 2 3]]"
    "[+ 1 [+ 2 3]]" "  [ +   1   [+   2 3   ]   ]  "
    "([])"          "([])"))

(deftest read-hash-maps
  (are [printed src] (= printed (pr-str (read-string src)))
    "{}"                    "{}"
    "{}"                    "{ }"
    "{\"abc\" 1}"           "{\"abc\" 1}"
    "{\"a\" {\"b\" 2}}"     "{\"a\" {\"b\" 2}}"
    "{\"a\" {\"b\" {\"c\" 3}}}" "{\"a\" {\"b\" {\"c\" 3}}}"
    "{\"a\" {\"b\" {\"cde\" 3}}}" "{  \"a\"  {\"b\"   {  \"cde\"     3   }  }}"
    "{:a {:b {:cde 3}}}"    "{  :a  {:b   {  :cde     3   }  }}"
    "{\"1\" 1}"             "{\"1\" 1}"
    "({})"                  "({})")
  ;; multi-key print order is unstable: compare as values instead
  (is (= {"a1" 1 "a2" 2 "a3" 3} (read-string "{\"a1\" 1 \"a2\" 2 \"a3\" 3}"))))

(deftest read-comments
  (are [printed src] (= printed (pr-str (read-string src)))
    "1" "1 ; comment after expression"
    "1" "1; comment after expression"))

(deftest read-deref-macro
  (is (= "(deref a)" (pr-str (read-string "@a")))))

;; -------- Optional Functionality --------

(deftest read-metadata-macro
  (is (= "(with-meta [1 2 3] {\"a\" 1})" (pr-str (read-string "^{\"a\" 1} [1 2 3]")))))

(deftest read-string-escapes
  (are [printed src] (= printed (pr-str (read-string src)))
    "\"\\n\""  "\"\\n\""
    "\"#\""    "\"#\""
    "\"$\""    "\"$\""
    "\"%\""    "\"%\""
    "\".\""    "\".\""
    "\"\\\\\"" "\"\\\\\""
    "\"|\""    "\"|\""))
