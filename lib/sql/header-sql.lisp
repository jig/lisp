;; $MODULE header-sql

(do
  ;; (with-tx [tx db] body...) runs body inside a transaction on db, binding
  ;; the transaction to tx. It commits when body finishes and rolls back if
  ;; body throws.
  (defmacro with-tx
    (fn [binding & body]
      `(sql-transact ~(nth binding 1)
         (fn [~(nth binding 0)] ~@body))))

  ;; (with-open [name (sql-open ...)] body...) binds name to a freshly opened
  ;; handle and guarantees it is closed when body finishes or throws.
  (defmacro with-open
    (fn [binding & body]
      `(let [~(nth binding 0) ~(nth binding 1)]
         (try
           ~@body
           (finally (sql-close ~(nth binding 0))))))))
