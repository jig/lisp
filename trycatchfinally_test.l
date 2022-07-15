(try*
    true
        (catch* e
            (println 3)
            e)
        (finally*
            (println 4)
            (println 5)
            ))