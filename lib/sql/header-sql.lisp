;; $MODULE header-sql

(do
  ;; (with-tx [tx db] body...) runs body inside a transaction on db, binding
  ;; the transaction to tx. It commits when body finishes and rolls back if
  ;; body throws.
  (defmacro with-tx
    (with-meta
      (fn [binding & body]
        `(sql-transact ~(nth binding 1)
           (fn [~(nth binding 0)] ~@body)))
      {:doc "(with-tx [tx db] body…) runs body in a transaction bound to tx: commits on success, rolls back on throw."}))

  ;; (with-open [name (sql-open ...)] body...) binds name to a freshly opened
  ;; handle and guarantees it is closed when body finishes or throws.
  (defmacro with-open
    (with-meta
      (fn [binding & body]
        `(let [~(nth binding 0) ~(nth binding 1)]
           (try
             ~@body
             (finally (sql-close ~(nth binding 0))))))
      {:doc "(with-open [name (sql-open …)] body…) binds name and guarantees sql-close when body finishes or throws."})))
