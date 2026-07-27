;; The .state/ store: state-save writes canonical lisp data and commits
;; it in the same operation; state-load reads it back as pure data and,
;; under lisp-integrity, requires it to match its committed version at
;; HEAD. State commits advance HEAD as children of the release, so
;; every restart keeps verifying the same code.
;; With /etc/lisp/allowed_signers present the state commit is SSH-signed
;; automatically with the ssh-agent key listed there — no key in the code.
(assert-integrity)

(def db (state-load "db" {:visits 0}))
(def db (assoc db :visits (+ 1 (get db :visits))))
(state-save "db" db)

(println "visit number" (get db :visits))
