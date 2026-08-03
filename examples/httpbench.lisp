;; A miniature ApacheBench on futures + loop/recur: C concurrent
;; workers (goroutines on every core) issue N requests total, with
;; keep-alive for free (Go's HTTP client pools connections per host).
;; Run examples/httpserver.lisp first, then:
;;   lisp examples/httpbench.lisp 32 20000
(def c (if (empty? *ARGV*) 8 (read-string (first *ARGV*))))

(def n (if (< (count *ARGV*) 2) 10000 (read-string (nth *ARGV* 1))))

(def url (if (< (count *ARGV*) 3) "http://localhost:8080/" (nth *ARGV* 2)))

(defn worker
  "Issues reqs GETs against url; returns {:ok :err :lat} with one
    latency (µs) per request."
  [reqs url]
  (loop [i 0 ok 0 err 0 lat []]
    (if (= i reqs)
      {:ok ok :err err :lat lat}
      (let [t0 (time-ns)
            resp (try (web-get url) (catch e nil))
            us (quot (- (time-ns) t0) 1000)]
        (recur (inc i)
          (if (and resp (= 200 (get resp :status))) (inc ok) ok)
          (if resp err (inc err))
          (conj lat us))))))

(def t0 (time-ms))

(def workers (map (fn [_] (future (worker (quot n c) url))) (range 0 c)))

(def results (map deref workers))

(def elapsed (- (time-ms) t0))

(def all-lat (sort (reduce concat [] (map (fn [r] (get r :lat)) results))))

(defn pct [p] (nth all-lat (quot (* p (count all-lat)) 100)))

(printf "%d workers, %d requests in %d ms → %d req/s\n"
  c (count all-lat) elapsed (quot (* 1000 (count all-lat)) elapsed))

(printf "ok %d, errors %d\n"
  (reduce (fn [a r] (+ a (get r :ok))) 0 results)
  (reduce (fn [a r] (+ a (get r :err))) 0 results))

(printf "latency µs: p50 %d  p90 %d  p99 %d  max %d\n"
  (pct 50) (pct 90) (pct 99) (nth all-lat (dec (count all-lat))))
