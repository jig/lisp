(def app
  (web-router
    [["/" {:get (fn [req] (web-text "OK"))}]]))

(web-serve {:port 8080 :handler app})
