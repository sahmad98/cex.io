# cex.io
[![Build Status](https://travis-ci.org/sahmad98/cex.io.svg?branch=master)](https://travis-ci.org/sahmad98/cex.io)

Unofficial cex.io websockets library in golang

## Benchmarks
```
goos: darwin
goarch: amd64
pkg: github.com/sahmad98/cex.io
BenchmarkRingBufferGetPut-4       	50000000	        39.8 ns/op
BenchmarkUpdateTicker-4           	10000000	       190 ns/op
BenchmarkParseFloat-4             	30000000	        44.8 ns/op
BenchmarkOrderbookLookup-4        	100000000	        15.8 ns/op
BenchmarkOrderbookRemoveLevel-4   	100000000	        22.3 ns/op
BenchmarkOrderbookUpdateLevel-4   	100000000	        15.1 ns/op
PASS
ok  	github.com/sahmad98/cex.io	10.997s
```
