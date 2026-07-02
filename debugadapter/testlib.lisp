;; testlib.lisp

;; helper library loaded by test.lisp to exercise cross-file debugging:
;; F11 on a (cube …) call in test.lisp must land inside this file

(defn cube [x]
    (* x (sqr-local x)))

(defn sqr-local [x]
    (* x x))
