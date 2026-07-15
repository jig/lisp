;; Replica of tests/stepK_numbers.mal: number literals, radix big ints,
;; floats and the variadic arithmetic. Literal-to-print expectations
;; (;=>) become pr-str comparisons; read errors go through read-string.

(deftest numbers-decimal-ints
  (are [expected expr] (= expected expr)
    -1    -1
    -1000 -1000
    -1000 -1_000
    1000  1_0_0_0
    0     0
    1     1
    9     9
    42    42
    1234567890 1234567890))

(deftest numbers-leading-zero-octal-rejected
  (is (= :threw (try (read-string "042") (catch e :threw))))
  (is (= :threw (try (read-string "00") (catch e :threw)))))

(deftest numbers-radix-literals-are-big-ints
  ;; radix literals print back as 0x… padded to whole octets
  (are [printed literal] (= printed (pr-str literal))
    "0xCAFECAFE" 0xCAFE_CAFE
    "0xCAFECAFE" 0xCA_FE_CA_FE
    "0x00"       0x0
    "0x01"       0x1
    "0x0F"       0xf
    "0x42"       0x42
    "0x0123456789ABCDEF" 0x123456789abcDEF
    "0x0F"       0o17
    "0x05"       0b101
    ;; beyond the machine word without overflow
    "0xFFFFFFFFFFFFFFFFFFFF" 0xFFFF_FFFF_FFFF_FFFF_FFFF
    ;; signed radix literals round-trip too
    "-0x01"      -0x01
    "-0xCAFECAFE" -0xCAFE_CAFE)
  ;; numeric equality, not pointer identity; distinct from machine ints
  (is (= 0x0A 0x0A))
  (is (= false (= 0x0A 10)))
  (is (= -0x01 -0x01)))

(deftest numbers-floats-print
  (are [printed literal] (= printed (pr-str literal))
    "3.1416"       3.1416
    "0"            0.
    "1"            1.
    "42"           42.
    "1.234568e+09" 01234567890.
    "0"            .0
    "0.1"          .1
    "0.42"         .42
    "0.012345679"  .0123456789
    "0"            0.0
    "1"            1.0
    "42"           42.0
    "0"            0e0
    "1"            1e0
    "42"           42e0
    "0"            0E0
    "42"           42E0
    "0"            0e+10
    "1e-10"        1e-10
    "4.2e+11"      42e+10
    "0.12345679"   01234567890e-10
    "1e-10"        1E-10
    "4.2e+11"      42E+10))

(deftest numbers-variadic-arithmetic
  (are [expected expr] (= expected expr)
    0  (+)
    5  (+ 5)
    3  (+ 1 1 1)
    15 (+ 1 2 3 4 5)
    1  (*)
    24 (* 2 3 4)
    -10 (- 10)
    7  (- 10 1 2)
    2  (/ 12 2 3)
    3  (/ 7 2)
    0  (/ 2)
    1  (/ 1))
  (is (= :threw (try (/ 0) (catch e :threw))))
  (is (= :threw (try (/ 1 0) (catch e :threw)))))

(deftest numbers-float-tower
  ;; a float makes the result float: compare against float literals
  ;; ((= 4 4.0) is false — machine ints and floats are distinct types)
  (are [expected expr] (= expected expr)
    4.0 (+ 1.5 2.5)
    1.5 (+ 1 0.5)
    3.0 (* 2 1.5)
    0.5 (- 1 0.25 0.25))
  (is (= false (= 4 4.0))))

(deftest numbers-bigint-arithmetic
  (are [printed expr] (= printed (pr-str expr))
    "0x03"   (+ 0x01 0x02)
    "0x02"   (+ 0x01 1)
    "-0x01"  (- 0x01 2)
    "-0xFF"  (- 0xFF)
    "0x0100" (* 0x10 0x10)
    "0x05"   (/ 0x10 0x03)
    "0x0FFFFFFFFFFFFFFFF0" (* 0xFFFF_FFFF_FFFF_FFFF 0x10))
  ;; big ints and floats do not mix
  (is (= :threw (try (+ 0x01 1.5) (catch e :threw)))))

(deftest numbers-ordering-chains
  (are [expected expr] (= expected expr)
    true  (< 1 2 3)
    false (< 1 3 2)
    true  (< 5)
    true  (<= 1 1 2)
    true  (> 3 2 1)
    true  (>= 3 3 1)
    true  (< 1 1.5 2)
    true  (< 0x01 0x02)
    true  (< 0x01 2)))
