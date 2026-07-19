;; The .state/ store: state-save writes canonical lisp data and commits
;; it in the same operation; state-load reads it back as pure data and,
;; under --integrity, requires it to match its committed version at
;; HEAD. The state commits do not invalidate the code ref: every
;; restart uses the same --integrity argument.
(assert-integrity)

(def db (state-load "db" {:visits 0}))
(def db (assoc db :visits (+ 1 (get db :visits))))
(state-save "db" db)

(println "visit number" (get db :visits))
