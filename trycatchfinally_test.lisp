(try
    (println 1)
    (println 2)
    (println 3)
    (throw true)
    (catch e
        (println 4)
        (println 5)
        (println 6)
        e)
    (finally
        (println 7)
        (println 8)
        (println 9)))
