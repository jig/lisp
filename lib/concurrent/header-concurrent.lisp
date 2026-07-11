;; $MODULE header-concurrent

(defmacro future (with-meta (fn [& body]
    `(^{:once true} future-call (fn [] ~@body)))
    {:doc "Runs body on its own goroutine, returning a future; deref (or @) blocks for its result."}))
