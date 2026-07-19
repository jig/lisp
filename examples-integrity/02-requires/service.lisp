;; The integrity check cascades to code loaded at runtime: this require
;; resolves to .lisp/util.lisp in the same repository, which must
;; byte-match its committed blob too. Editing util.lisp without
;; committing makes this program refuse to start.
(require "util")
(assert-integrity)
(println (util/greet "operator"))
