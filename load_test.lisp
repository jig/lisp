(do
    (def  hm (-> {}
        (assoc :a 1)
        (assoc :b 2)))
    (assert
        (=
            3
            (+ (get hm :a) (get hm :b))))
    (get hm :b))
