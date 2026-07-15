;; Replica of tests/stepM_take_drop.mal: take, drop, take-last,
;; drop-last and subvec.

(deftest take-basics
  (are [expected expr] (= expected expr)
    (quote (1 2 3)) (take 3 (quote (1 2 3 4 5 6)))
    (quote (1 2 3)) (take 3 [1 2 3 4 5 6])
    (quote (1 2))   (take 3 [1 2])
    ()              (take 1 [])
    ()              (take 1 nil)
    ()              (take 0 [1])
    ()              (take -1 [1])))

(deftest drop-basics
  (are [expected expr] (= expected expr)
    (quote (1 2 3 4)) (drop -1 [1 2 3 4])
    (quote (1 2 3 4)) (drop 0 [1 2 3 4])
    (quote (3 4))     (drop 2 [1 2 3 4])
    ()                (drop 5 [1 2 3 4]))
  ;; similar to subvec but with seqs
  (is (= (quote (6 7 8)) (take 3 (drop 5 (range 1 11))))))

(deftest take-last-basics
  (are [expected expr] (= expected expr)
    (quote (3 4)) (take-last 2 [1 2 3 4])
    (quote (4))   (take-last 2 [4])
    nil           (take-last 2 [])
    nil           (take-last 2 nil)
    nil           (take-last 0 [1])
    nil           (take-last -1 [1])))

(deftest drop-last-basics
  ;; default n=1 is unsupported
  (is (= :threw (try (drop-last [1 2 3 4]) (catch e :threw))))
  (are [expected expr] (= expected expr)
    (quote (1 2 3 4)) (drop-last -1 [1 2 3 4])
    (quote (1 2 3 4)) (drop-last 0 [1 2 3 4])
    ()                (drop-last 5 [1 2 3 4])
    (quote (1 2))     (drop-last 2 [1 2 3 4])
    (quote (1 2))     (drop-last 2 (quote (1 2 3 4))))
  ;; unsupported with hash-maps
  (is (= :threw (try (drop-last 2 {:a 1 :b 2 :c 3 :d 4}) (catch e :threw)))))

(deftest subvec-basics
  (is (= [3 4 5 6 7] (subvec [1 2 3 4 5 6 7] 2)))
  (is (= [3 4] (subvec [1 2 3 4 5 6 7] 2 4))))
