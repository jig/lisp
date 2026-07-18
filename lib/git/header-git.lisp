;; $MODULE header-git

(do
  ;; (git-with-repo [r (git-open ...)] body...) binds r to a repository
  ;; handle and guarantees it is closed when body finishes or throws.
  (defmacro git-with-repo
    (with-meta
      (fn [binding & body]
        `(let [~(nth binding 0) ~(nth binding 1)]
           (try
             ~@body
             (finally (git-close ~(nth binding 0))))))
      {:doc "(git-with-repo [r (git-open …)] body…) binds r and guarantees git-close when body finishes or throws."}))

  ;; Boolean conveniences over the fail-closed verify builtins.
  (def git-verified?
    (with-meta
      (fn [repo rev allowed-keys]
        (try
          (do (git-verify-commit repo rev allowed-keys) true)
          (catch e false)))
      {:doc "(git-verified? repo rev allowed-keys) is true when the commit's SSH signature verifies against allowed-keys."}))

  (def git-tag-verified?
    (with-meta
      (fn [repo name allowed-keys]
        (try
          (do (git-verify-tag repo name allowed-keys) true)
          (catch e false)))
      {:doc "(git-tag-verified? repo name allowed-keys) is true when the tag's SSH signature verifies against allowed-keys."})))
