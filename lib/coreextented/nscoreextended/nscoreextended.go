package nscoreextended

import (
	"github.com/jig/mal"
	. "github.com/jig/mal/types"
)

func Load(repl_env EnvType) error {
	if _, err := mal.REPL(repl_env, `(eval (read-string (str "(do "`+BasicsFuncsAndMacros+`" nil)"))))`, nil); err != nil {
		return err
	}
	return nil
}

var BasicsFuncsAndMacros = `;; prerequisites
;; Trivial but convenient functions.

;; Integer predecessor (number -> number)
(def! inc (fn* [a] (+ a 1)))

;; Integer predecessor (number -> number)
(def! dec (fn* (a) (- a 1)))

;; Integer nullity test (number -> boolean)
(def! zero? (fn* (n) (= 0 n)))

;; Returns the unchanged argument.
(def! identity (fn* (x) x))

;; Generate a hopefully unique symbol. See section "Plugging the Leaks"
;; of http://www.gigamonkeys.com/book/macros-defining-your-own.html
(def! gensym
  (let* [counter (atom 0)]
    (fn* []
      (symbol (str "G__" (swap! counter inc))))))

;; Left and right folds.

;; Left fold (f (.. (f (f init x1) x2) ..) xn)
(def! reduce
  (fn* (f init xs)
    ;; f      : Accumulator Element -> Accumulator
    ;; init   : Accumulator
    ;; xs     : sequence of Elements x1 x2 .. xn
    ;; return : Accumulator
    (if (empty? xs)
      init
      (reduce f (f init (first xs)) (rest xs)))))

;; Right fold (f x1 (f x2 (.. (f xn init)) ..))
;; The natural implementation for 'foldr' is not tail-recursive, and
;; the one based on 'reduce' constructs many intermediate functions, so we
;; rely on efficient 'nth' and 'count'.
(def! foldr
  (let* [
    rec (fn* [f xs acc index]
      (if (< index 0)
        acc
        (rec f xs (f (nth xs index) acc) (- index 1))))
    ]

    (fn* [f init xs]
      ;; f      : Element Accumulator -> Accumulator
      ;; init   : Accumulator
      ;; xs     : sequence of Elements x1 x2 .. xn
      ;; return : Accumulator
      (rec f xs init (- (count xs) 1)))))

;; Search for first evaluation returning 'nil' or 'false'.
;; Rewrite 'x1 x2 .. xn x' as
;;   (let* [r1 x1]
;;     (if r1 test1
;;       (let* [r2 x2]
;;         ..
;;         (if rn
;;           x
;;           rn) ..)
;;       r1))
;; Without arguments, returns 'true'.
(defmacro! and
  (fn* (& xs)
    ;; Arguments and the result are interpreted as boolean values.
    (cond (empty? xs)      true
          (= 1 (count xs)) (first xs)
          true             (let* (condvar (gensym))
                            ` + "`" + `(let* (~condvar ~(first xs))
                              (if ~condvar (and ~@(rest xs)) ~condvar))))))
`
