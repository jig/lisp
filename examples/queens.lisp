;; The eight queens puzzle in jig/lisp: place N queens on an N×N board so
;; that none attacks another. Solved by column-by-column backtracking.
;;
;; A partial placement is a list of rows, one per column already placed,
;; most-recent column first. So the queen `d` columns to the left of the
;; one being tried sits at (nth queens (- d 1)).
;;
;; Run it with:  lisp examples/queens.lisp

(defn abs
  "Absolute value of n."
  [n]
  (if (< n 0) (- 0 n) n))

(defn safe?
  ¬Whether a queen on `row` is safe against the already-placed `queens`
  (no shared row and no shared diagonal).¬
  [queens row]
  (every?
    (fn [i]
      (let [placed   (nth queens i)
            distance (+ i 1)]
        (if (= placed row)
          false
          (not (= (abs (- placed row)) distance)))))
    (range 0 (count queens))))

(defn solutions
  "All safe completions of `queens` on an n×n board, each a list of rows."
  [n queens]
  (if (= (count queens) n)
    (list queens)
    (reduce
      (fn [acc row]
        (if (safe? queens row)
          (concat acc (solutions n (cons row queens)))
          acc))
      (list)
      (range 0 n))))

(defn render
  "Render a solution (a list of rows) as an ASCII board."
  [queens]
  (let [n (count queens)]
    (reduce
      (fn [board col]
        (let [row (nth queens col)]
          (str board
            (reduce
              (fn [line c] (str line (if (= c row) " Q" " .")))
              ""
              (range 0 n))
            "\n")))
      ""
      (range 0 n))))

;; main
(def board-size 8)
(def all (solutions board-size (list)))

(println (str board-size " queens: " (count all) " solutions"))
(println)
(println "First solution:")
(println (render (first all)))
