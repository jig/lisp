;; $MODULE header-include

(defn require [module]
  (load-file-once (resolve-require module)))
