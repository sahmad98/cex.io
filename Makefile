all: schema examples

.PHONY: schema
schema:
	$(info Creating flatbuffer files)
	flatc --go schema/orderbook.fbs
	flatc --python schema/orderbook.fbs
	mkdir -p types/cexio
	mv types/*.py types/cexio/

.PHONY: check_flat_compiler
check_flat_compiler:
	ifeq ($(shell which flatc), )
	$(error No Flatbuffer compiler in path. Check installation.)
	endif

.PHONY: clean
clean:
	rm types/*.go
	rm types/cexio/*.py

.PHONY: examples
examples:
	$(info )
	$(info Building Examples)
	go build -pkgdir examples
