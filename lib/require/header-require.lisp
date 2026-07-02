;; $MODULE header-require

;; Depends on load-file-once (lib/coreextented): load that library first.
(defn require [module]
  (load-file-once (resolve-require module)))
