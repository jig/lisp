;; Minimal HTTP client. Run examples/httpserver.lisp first, then:
;;   lisp examples/httpclient.lisp
(def resp (web-get "http://localhost:8080/"))

(println (get resp :status) (get resp :body))
