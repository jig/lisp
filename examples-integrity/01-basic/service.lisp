;; Minimal integrity-mode program: assert-integrity throws unless the
;; interpreter is lisp-integrity, and returns the verified commit
;; hash — so this line both demands the mode and logs the release.
(println "running release:" (assert-integrity))
(println "hello from verified code")
