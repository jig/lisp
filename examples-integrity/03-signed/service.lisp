;; With --integrity-signers, the ref must carry an SSH signature by a
;; trusted key: the trust anchor becomes the key list, not the local
;; repository state. See the README for creating the signed tag.
(println "running release signed by a trusted key:" (assert-integrity))
