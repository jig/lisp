;; cli — command-line option parsing for jig/lisp, modelled on
;; clojure/tools.cli (a pragmatic subset). Pure Lisp.
;;
;; (cli-parse-opts args specs) parses a sequence of argument strings
;; (typically *ARGV*) against a vector of option specs and returns a map
;;   {:options   {…}      ; parsed option values, keyed by :id
;;    :arguments [...]     ; leftover positional arguments
;;    :summary   "…"       ; a help string built from the specs
;;    :errors    nil|[…]}  ; error messages, or nil when all is well
;; Read fields with get, e.g. (get result :options).
;;
;; A spec is a vector [short long desc & kv-opts]:
;;   short   short flag string, e.g. "-p" (or nil)
;;   long    long flag string, e.g. "--port PORT" (with an argument name)
;;           or "--verbose" (no argument name → boolean flag)
;;   desc    one-line description
;; kv-opts (optional):
;;   :default val         value when the option is absent
;;   :parse-fn fn          applied to the raw string, e.g. read-string
;;   :validate [pred msg]  pred is called on the parsed value; msg on fail
;;   :id keyword           override the derived id (default: long w/o "--")
;;
;; Subset limitations (vs tools.cli): only the separate "--opt val" form
;; (no "--opt=val"), no short grouping ("-abc"), no :update-fn/:multi,
;; no in-order/sub-command parsing.
(do
  (defn cli--compile-spec [spec]
    (let [short (nth spec 0)
          long  (nth spec 1)
          desc  (nth spec 2)
          kvs   (apply hash-map (drop 3 spec))
          parts (split long " ")
          flag  (nth parts 0)
          takes (= (count parts) 2)]
      (hash-map
        :short       short
        :long        flag
        :arg-name    (if takes (nth parts 1) nil)
        :desc        desc
        :takes-arg   takes
        :id          (if (contains? kvs :id) (get kvs :id) (keyword (subs flag 2)))
        :has-default (contains? kvs :default)
        :default     (get kvs :default)
        :parse-fn    (get kvs :parse-fn)
        :validate    (get kvs :validate))))

  (defn cli--find-spec [compiled tok]
    (first (filter
             (fn [s] (or (= (get s :long) tok) (= (get s :short) tok)))
             compiled)))

  ;; Apply parse-fn then validate. Returns [ok? value-or-message].
  (defn cli--coerce [spec raw]
    (let [pf  (get spec :parse-fn)
          val (if pf (pf raw) raw)
          v   (get spec :validate)]
      (if v
        (if ((nth v 0) val)
          [true val]
          [false (str (get spec :long) ": " (nth v 1))])
        [true val])))

  (defn cli--defaults [compiled]
    (reduce
      (fn [m s] (if (get s :has-default) (assoc m (get s :id) (get s :default)) m))
      {}
      compiled))

  (defn cli--option-token? [tok]
    (and (starts-with? tok "-") (not (= tok "-")) (not (= tok "--"))))

  (defn cli-summarize
    "Builds a help string from a vector of option specs, one line per option."
    [specs]
    (reduce
      (fn [acc s]
        (str acc
             "  " (if (get s :short) (get s :short) "  ")
             " " (get s :long)
             (if (get s :arg-name) (str " " (get s :arg-name)) "")
             "  " (get s :desc) "\n"))
      ""
      (map cli--compile-spec specs)))

  (defn cli-parse-opts
    ¬Parses args (typically *ARGV*) against a vector of option specs.
Returns a map:
  :options   parsed values keyed by :id
  :arguments leftover positional arguments
  :summary   generated help text
  :errors    a vector of messages, or nil when all is well¬
    [args specs]
    (let [compiled (map cli--compile-spec specs)]
      (loop [toks    args
             opts    (cli--defaults compiled)
             posargs []
             errs    []
             no-opts false]
        (if (empty? toks)
          (hash-map
            :options   opts
            :arguments posargs
            :summary   (cli-summarize specs)
            :errors    (if (empty? errs) nil errs))
          (let [tok  (first toks)
                more (rest toks)]
            (cond
              no-opts
                (recur more opts (conj posargs tok) errs true)
              (= tok "--")
                (recur more opts posargs errs true)
              (cli--option-token? tok)
                (let [spec (cli--find-spec compiled tok)]
                  (cond
                    (nil? spec)
                      (recur more opts posargs (conj errs (str "Unknown option: " tok)) no-opts)
                    (get spec :takes-arg)
                      (if (empty? more)
                        (recur more opts posargs (conj errs (str "Missing argument for " tok)) no-opts)
                        (let [res (cli--coerce spec (first more))]
                          (if (nth res 0)
                            (recur (rest more) (assoc opts (get spec :id) (nth res 1)) posargs errs no-opts)
                            (recur (rest more) opts posargs (conj errs (nth res 1)) no-opts))))
                    :else
                      (recur more (assoc opts (get spec :id) true) posargs errs no-opts)))
              :else
                (recur more opts (conj posargs tok) errs no-opts)))))))
)
