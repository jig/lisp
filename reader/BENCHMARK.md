Pre removal of regexp:

```bash
❯ go test -bench=. ./... -benchtime=10s
goos: darwin
goarch: arm64
pkg: github.com/jig/lisp
BenchmarkLoadSymbols-10                          	  297692	     38451 ns/op
BenchmarkMAL1-10                                 	  217472	     54719 ns/op
BenchmarkMAL2-10                                 	 1615966	      7416 ns/op
BenchmarkParallelREAD-10                         	 3756685	      3244 ns/op
BenchmarkParallelREP-10                          	 2838428	      4265 ns/op
BenchmarkREP-10                                  	 1597930	      7488 ns/op
BenchmarkFibonacci-10                            	   30750	    388149 ns/op
BenchmarkParallelFibonacci-10                    	   58689	    203702 ns/op
BenchmarkAtomParallel-10                         	 4544056	      2621 ns/op
BenchmarkAddPreamble-10                          	12085546	       989.0 ns/op
BenchmarkAddPreambleAlternative-10               	 3712512	      3228 ns/op
BenchmarkREADWithPreamble-10                     	  459700	     25936 ns/op
BenchmarkNewEnv-10                               	 8883668	      1328 ns/op
BenchmarkCompleteSendingWithPreamble-10          	  127327	     94265 ns/op
BenchmarkCompleteSendingWithPreambleSolved-10    	   91657	    130333 ns/op
PASS
```