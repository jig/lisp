module github.com/jig/lisp

go 1.25.0

// v0.3.0 was tagged prematurely (the 0.3 line was not ready) and
// v0.3.1 exists only to carry this retraction. Keep both retracted in
// every future release so they stay hidden from version listings.
retract (
	v0.3.0 // Published prematurely; use v0.2.24.
	v0.3.1 // Contains only this retraction.
)

require (
	github.com/MicahParks/keyfunc/v3 v3.8.0
	github.com/alexflint/go-arg v1.6.1
	github.com/chzyer/readline v1.5.1
	github.com/davecgh/go-spew v1.1.1
	github.com/go-git/go-billy/v6 v6.0.0-alpha.1
	github.com/go-git/go-git/v6 v6.0.0-alpha.4
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/hiddeco/sshsig v0.2.0
	github.com/jackc/pgx/v5 v5.10.0
	github.com/jig/scanner v1.3.1
	golang.org/x/crypto v0.54.0
	golang.org/x/term v0.45.0
	modernc.org/sqlite v1.53.0
)

require (
	github.com/MicahParks/jwkset v0.11.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.4.1 // indirect
	github.com/alexflint/go-scalar v1.2.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/go-git/gcfg/v2 v2.0.2 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/kevinburke/ssh_config v1.6.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/pjbgf/sha1cd v0.6.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/sergi/go-diff v1.4.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	modernc.org/libc v1.73.4 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)
