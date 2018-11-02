all: schema

.PHONY: schema
schema:
	echo "Creating Golang Flatbuffer Libs"
	flatc --go schema/orderbook.fbs
	echo "Creating Python Flatbuffers Libs"
	flatc --python schema/orderbook.fbs
	echo "Creating Python Directory"
	mkdir -p types/cexio
	echo "Moving Python Flatbuffers file"
	mv types/*.py types/cexio/

.PHONY: check_flat_compiler
check_flat_compiler:
	ifeq ($(shell which flatc), )
	$(error "No Flatbuffer compiler in path. Check installation.")
	endif

.PHONY: clean
clean:
	rm types/*.go
	rm types/cexio/*.py
