;; testlib.lisp

;; helper library loaded by debugadapter/test.lisp via (require "testlib")
;; to exercise cross-file debugging: F11 on a (cube …) call must land
;; inside this file. It lives in <git root>/.lisp/, one of the standard
;; require search directories.

(defn cube [x]
    (* x (sqr-local x)))

(defn sqr-local [x]
    (* x x))
