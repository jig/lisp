;; Replica of tests/stepL_envvar.mal: environment variables.
;; The original's PWD-basename check is omitted: it depends on the
;; working directory the runner happens to be invoked from.

(deftest envvar-set-get-unset
  (is (= nil (setenv "DEFTEST_VAR1" "hello")))
  (is (= nil (setenv "DEFTEST_VAR2" "")))
  (is (= "hello" (getenv "DEFTEST_VAR1")))
  (is (= "" (getenv "DEFTEST_VAR2")))
  (is (= nil (getenv "DEFTEST_VAR3")))
  (is (= nil (unsetenv "DEFTEST_VAR1")))
  (is (= nil (getenv "DEFTEST_VAR1")))
  (unsetenv "DEFTEST_VAR2"))
