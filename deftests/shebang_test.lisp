#!/usr/bin/env lisp
;; This file is itself a shebang script: loading it through the test
;; runner proves load-file treats the #! line as a comment.

(deftest shebang-is-a-comment
  (is (= 7 (read-string "#!/usr/bin/env lisp\n7"))))

(deftest read-program-builtin
  (is (= "(do 1 2 3)" (pr-str (read-program "1 2 3" "inline"))))
  (is (= 3 (eval (read-program "(def shebang-x 1)\n(+ shebang-x 2)" "inline"))))
  (is (= nil (read-program ";; only a comment" "inline"))))
