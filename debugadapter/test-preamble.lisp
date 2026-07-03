;; $GREETING "high by default"
;; $FACTOR 1
;; test-preamble.lisp — exercises preamble placeholders under the
;; debugger. The two `;; $NAME <expr>` lines above are in-file defaults;
;; the launch configuration "LISP DEBUGGER: PREAMBLE TEST" overrides
;; them with --preamble flags. Run it plain (defaults) or with:
;;
;;   lisp-debug -P '$FACTOR 1984' -P '$GREETING "hola flags"' debugadapter/test-preamble.lisp

(println $GREETING)

(def doubled (* $FACTOR 2))
(println doubled)

(if (> $FACTOR 1)
    (println "factor overridden by --preamble")
    (println "factor from the in-file default"))
