# Generate and verify the Omada northbound operation index.
#
# gen    - fetch the live spec, assert the pinned fingerprint in
#          gen/data/spec.sha256, and write internal/opsindex/ops_index.gen.go
#          (deterministic, gofmt-clean).
# verify - run gen, then require the generated file to be gofmt-clean and
#          the opsindex tests to pass.

.PHONY: gen verify

gen:
	python3 gen/genopsindex.py

verify: gen
	@out=$$(gofmt -l internal/opsindex/ops_index.gen.go); \
	if [ -n "$$out" ]; then \
		echo "gofmt: $${out} needs formatting"; exit 1;
	fi
	go test ./internal/opsindex/ -race -count=1
