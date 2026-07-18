(do
;;; Trivial
  ;; Trivial but convenient functions.
  ;; NOTE: inc, dec, gensym — and the when/and/or macros and
  ;; load-file-once below — moved to the core headers (header-basic,
  ;; header-load-file) so every library header can rely on them with
  ;; only core loaded.

  (defn zero?
    "Whether n equals 0."
    [n]
    (= 0 n))

  (defn identity
    "Returns its argument unchanged."
    [x]
    x)

;;; Benchmark
  ;; An alternative approach, to complement perf.mal
  ;; requires Trivial

  (defn benchmark*
    "Runs f n times, collecting the elapsed milliseconds of each run (helper for benchmark)."
    [f n results]
    (if (< 0 n)
      (let [start-ms (time-ms)
            _ (f)
            end-ms (time-ms)]
        (benchmark* f (- n 1) (conj results (- end-ms start-ms))))
      results))

  (defmacro benchmark
    (with-meta
      (fn [expr n]
        `(benchmark* (fn [] ~expr) ~n []))
      {:doc "Evaluates expr n times, returning a vector with the elapsed milliseconds of each run."}))

;;; Reducers
  ;; Left and right folds.

  (defn reduce
    "Left fold: (f (.. (f (f init x1) x2) ..) xn) over the elements of xs."
    [f init xs]
    ;; f      : Accumulator Element -> Accumulator
    ;; init   : Accumulator
    ;; xs     : sequence of Elements x1 x2 .. xn
    ;; return : Accumulator
    (if (empty? xs)
      init
      (reduce f (f init (first xs)) (rest xs))))

  (defn reduce-kv
    "Left fold over a sequence of key/value pairs: applies (f acc k v) across xs."
    [f init xs]
    ;; f      : Accumulator Element -> Accumulator
    ;; init   : Accumulator
    ;; xs     : sequence of key-value pairs k1-v1 k2-v2...
    ;; return : Accumulator
    (if (empty? xs)
      init
      (reduce-kv f (f init (nth xs 0) (nth xs 1)) (rest (rest xs)))))

  ;; The natural implementation for 'foldr' is not tail-recursive, and
  ;; the one based on 'reduce' constructs many intermediate functions, so we
  ;; rely on efficient 'nth' and 'count'.
  (def foldr
    (with-meta
      (let [
        rec (fn [f xs acc index]
          (if (< index 0)
            acc
            (rec f xs (f (nth xs index) acc) (- index 1))))
        ]
        (fn [f init xs]
          ;; f      : Element Accumulator -> Accumulator
          ;; init   : Accumulator
          ;; xs     : sequence of Elements x1 x2 .. xn
          ;; return : Accumulator
          (rec f xs init (- (count xs) 1))))
      {:doc "Right fold: (f x1 (f x2 (.. (f xn init)))) over the elements of xs."}))

;;; Partial Application
  ;; Partial application of functions.

  (defn partial
    "Returns a function that calls f with the given args plus any it is later called with."
    [f & args]
    (fn [& more-args]
      (apply f (concat args more-args))))

;;; Threading
  ;; Composition of partially applied functions.

  ;; Rewrite x (a a1 a2) .. (b b1 b2) as
  ;;   (b (.. (a x a1 a2) ..) b1 b2)
  ;; If anything else than a list is found were "(a a1 a2)" is expected,
  ;; replace it with a list with one element, so that "-> x a" is
  ;; equivalent to "-> x (list a)".
  (defmacro ->
    (with-meta
      (fn [x & xs]
        (reduce _iter-> x xs))
      {:doc "Thread-first: inserts each stage's result as the first argument of the next form."}))

  (defn _iter->
    "Threading helper for ->."
    [acc form]
    (if (list? form)
      `(~(first form) ~acc ~@(rest form))
      (list form acc)))

  ;; Like "->", but the arguments describe functions that are partially
  ;; applied with *left* arguments.  The previous result is inserted at
  ;; the *end* of the new argument list.
  ;; Rewrite x ((a a1 a2) .. (b b1 b2)) as
  ;;   (b b1 b2 (.. (a a1 a2 x) ..)).
  (defmacro ->>
    (with-meta
      (fn [x & xs]
        (reduce _iter->> x xs))
      {:doc "Thread-last: inserts each stage's result as the last argument of the next form."}))

  (defn _iter->>
    "Threading helper for ->>."
    [acc form]
    (if (list? form)
      `(~(first form) ~@(rest form) ~acc)
      (list form acc)))

;;; Memoize
  ;; For recursive functions, take care to store the wrapper under the
  ;; same name than the original computation with an assignment like
  ;; "(def f (memoize f))", so that intermediate results are memorized.
  ;; Adapted from http://clojure.org/atoms
  (defn memoize
    "Returns a caching version of f: results are stored by argument and reused."
    [f]
    (let [mem (atom {})]
      (fn [& args]
        (let [key (str args)]
          (if (contains? @mem key)
            (get @mem key)
            (let [ret (apply f args)]
              (do
                (swap! mem assoc key ret)
                ret)))))))

;;; Performance
  ;; Mesure performances.
  ;; requires trivial package

  (defmacro time
    (with-meta
      (fn [exp]
        (let [start (gensym)
              ret   (gensym)]
          `(let (~start (time-ms)
                  ~ret   ~exp)
            (do
              (println "Elapsed time:" (- (time-ms) ~start) "msecs")
              ~ret))))
      {:doc "Evaluates exp, prints the elapsed time, and returns its value."}))

  ;; Count evaluations of a function during a given time frame.
  (def run-fn-for
    (with-meta
      (let [
        run-fn-for* (fn [fn max-ms acc-ms last-iters]
          (let [start (time-ms)
                _ (fn)
                elapsed (- (time-ms) start)
                iters (inc last-iters)
                new-acc-ms (+ acc-ms elapsed)]
            (if (>= new-acc-ms max-ms)
              last-iters
              (run-fn-for* fn max-ms new-acc-ms iters))))
        ]

        (fn [fn max-secs]
          ;; fn       : function without parameters
          ;; max-secs : number (seconds)
          ;; return   : number (iterations)
          (do
            ;; Warm it up first
            (run-fn-for* fn 1000 0 0)
            ;; Now do the test
            (run-fn-for* fn (* 1000 max-secs) 0 0))))
      {:doc "Returns how many times the no-arg function fn runs in max-secs seconds (after a warm-up)."}))

;;; Pretty Print
  (def pprint
    (with-meta
      (let [

        spaces- (fn [indent]
          (if (> indent 0)
            (str " " (spaces- (- indent 1)))
            ""))

        pp-seq- (fn [obj indent]
          (let [xindent (+ 1 indent)]
            (apply str (pp- (first obj) 0)
                      (map (fn [x] (str "\n" (spaces- xindent)
                                          (pp- x xindent)))
                            (rest obj)))))

        pp-map- (fn [obj indent]
          (let [ks (keys obj)
                kindent (+ 1 indent)
                kwidth (count (seq (str (first ks))))
                vindent (+ 1 (+ kwidth kindent))]
            (apply str (pp- (first ks) 0)
                      " "
                      (pp- (get obj (first ks)) 0)
                      (map (fn [k] (str "\n" (spaces- kindent)
                                          (pp- k kindent)
                                          " "
                                          (pp- (get obj k) vindent)))
                            (rest ks)))))

        pp- (fn [obj indent]
          (cond
            (list? obj)   (str "(" (pp-seq- obj indent) ")")
            (vector? obj) (str "[" (pp-seq- obj indent) "]")
            (map? obj)    (str "{" (pp-map- obj indent) "}")
            :else         (pr-str obj)))

        ]

        (fn [obj]
            (println (pp- obj 0))))
      {:doc "Pretty-prints a lisp value with indentation."}))

;;; Protocols
  ;; A sketch of Clojure-like protocols, implemented in Mal
  ;; By chouser (Chris Houser)
  ;; Original: https://gist.github.com/Chouser/6081ea66d144d13e56fc

  ;; Most applications will override the default with an explicit value
  ;; for the ":type" key in the metadata.
  (defn find-type
    "Returns a keyword naming obj's type (overridable via :type metadata)."
    [obj]
    (cond
      (symbol?  obj) :mal/symbol
      (keyword? obj) :mal/keyword
      (atom?    obj) :mal/atom
      (nil?     obj) :mal/nil
      (true?    obj) :mal/boolean
      (false?   obj) :mal/boolean
      (number?  obj) :mal/number
      (string?  obj) :mal/string
      (macro?   obj) :mal/macro
      true
      (let [metadata (meta obj)
            type     (if (map? metadata) (get metadata :type))]
        (cond
          (keyword? type) type
          (list?   obj)   :mal/list
          (vector? obj)   :mal/vector
          (map?    obj)   :mal/map
          (fn?     obj)   :mal/function
          true            (throw "unknown MAL value in protocols")))))

  ;; A protocol (abstract class, interface..) is represented by a symbol.
  ;; It describes methods (abstract functions, contracts, signals..).
  ;; Each method is described by a sequence of two elements.
  ;; First, a symbol setting the name of the method.
  ;; Second, a vector setting its formal parameters.
  ;; The first parameter is required, plays a special role.
  ;; It is usually named "this" ("self"..).
  ;; For example,
  ;;   (defprotocol protocol
  ;;     (method1 [this])
  ;;     (method2 [this argument]))
  ;; can be thought as:
  ;;   (def method1 (fn [this]) ..)
  ;;   (def method2 (fn [this argument]) ..)
  ;;   (def protocol ..)
  ;; The return value is the new protocol.
  (defmacro defprotocol
    (with-meta
      (fn [proto-name & methods]
        ;; A protocol is an atom mapping a type extending the protocol to
        ;; another map from method names as keywords to implementations.
        (let [
          drop2 (fn [args]
            (if (= 2 (count args))
              ()
              (cons (first args) (drop2 (rest args)))))
          rewrite (fn [method]
            (let [
              name     (first method)
              args     (nth method 1)
              argc     (count args)
              varargs? (if (<= 2 argc) (= '& (nth args (- argc 2))))
              dispatch `(get (get @~proto-name
                                  (find-type ~(first args)))
                            ~(keyword (str name)))
              body     (if varargs?
                `(apply ~dispatch ~@(drop2 args) ~(nth args (- argc 1)))
                        (cons dispatch args))
              ]
              (list 'def name (list 'fn args body))))
          ]
            `(do
            ~@(map rewrite methods)
            (def ~proto-name (atom {})))))
      {:doc "Defines a protocol proto-name and its methods, dispatching on the argument's type."}))

  ;; A type (concrete class..) extends (is a subclass of, implements..)
  ;; a protocol when it provides implementations for the required methods.
  ;;   (extend type protocol {
  ;;     :method1 (fn [this] ..)
  ;;     :method2 (fn [this arg1 arg2])})
  ;; Additionnal protocol/methods pairs are equivalent to successive
  ;; calls with the same type.
  (defn extend
    "Registers a type's method implementations for a protocol (return value is nil)."
    [type proto methods & more]
    (do
      (swap! proto assoc type methods)
      (if (first more)
        (apply extend type more))))

  (defn satisfies?
    "Whether obj's type has been extended to protocol."
    [protocol obj]
    (contains? @protocol (find-type obj)))

;;; Test Cascade
  ;; Iteration on evaluations interpreted as boolean values.

  ;; "(or x1 x2 .. xn x)"
  (defn every?
    "Whether (pred x) is truthy for every x in xs."
    [pred xs]
    ;; pred   : Element -> interpreted as a logical value
    ;; xs     : sequence of Elements x1 x2 .. xn
    ;; return : boolean
    (cond (empty? xs)       true
          (pred (first xs)) (every? pred (rest xs))
          true              false))

  (defn some
    "Returns the first truthy (pred x) over xs, or nil."
    [pred xs]
    ;; pred   : Element -> interpreted as a logical value
    ;; xs     : sequence of Elements x1 x2 .. xn
    ;; return : the first truthy result, or nil
    (if (empty? xs)
      nil
      (or (pred (first xs))
          (some pred (rest xs)))))

;;; Arithmetic
  ;; Integer division helpers, absolute value and min/max. Integer `/`
  ;; already truncates toward zero, so quot is just a named alias.

  (defn quot
    "Integer quotient of a divided by b, truncated toward zero."
    [a b]
    (/ a b))

  (defn rem
    "Remainder of (quot a b); the sign follows the dividend a."
    [a b]
    (- a (* b (/ a b))))

  (defn mod
    "Modulo of a by b; the sign follows the divisor b."
    [a b]
    (let [r (rem a b)]
      (if (= r 0)
        0
        (if (if (< r 0) (< b 0) (> b 0)) ; r and b share sign?
          r
          (+ r b)))))

  (defn abs
    "Absolute value of n."
    [n]
    (if (< n 0) (- 0 n) n))

  (defn min
    "Smallest of one or more numbers."
    [a & more]
    (reduce (fn [m x] (if (< x m) x m)) a more))

  (defn max
    "Largest of one or more numbers."
    [a & more]
    (reduce (fn [m x] (if (> x m) x m)) a more))

;;; Numeric Predicates

  (defn pos?
    "Whether n is greater than 0."
    [n]
    (> n 0))

  (defn neg?
    "Whether n is less than 0."
    [n]
    (< n 0))

  (defn even?
    "Whether n is even."
    [n]
    (= 0 (mod n 2)))

  (defn odd?
    "Whether n is odd."
    [n]
    (not (even? n)))

;;; Sequence Filters
  ;; Eager filtering; the lazy counterparts live in lib/lazy.

  (defn filter
    "List of the items in xs for which (pred x) is truthy."
    [pred xs]
    (cond (empty? xs)       ()
          (pred (first xs)) (cons (first xs) (filter pred (rest xs)))
          true              (filter pred (rest xs))))

  (defn remove
    "List of the items in xs for which (pred x) is falsy."
    [pred xs]
    (filter (fn [x] (not (pred x))) xs))

  (defn take-while
    "Leading items of xs while (pred x) is truthy."
    [pred xs]
    (if (empty? xs)
      ()
      (if (pred (first xs))
        (cons (first xs) (take-while pred (rest xs)))
        ())))

;;; Collections

  (defn into
    "Pours every item of from into to using conj; the result keeps to's type."
    [to from]
    (reduce conj to from))

;;; Printing

  (defn printf
    "Prints (format fmt args...) without a trailing newline; returns nil."
    [fmt & args]
    (print (apply format fmt args)))

)
