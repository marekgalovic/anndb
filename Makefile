CC=clang
CC_FLAGS=-masm=intel -mno-red-zone -mstackrealign -mllvm -inline-threshold=1000 -fno-asynchronous-unwind-tables \
	-fno-exceptions -fno-rtti -fno-jump-tables -fno-builtin -fno-jump-tables -O3
AVX_FLAGS=-mavx
# SSE_FLAGS=

VERSION=0.0.2

compile_avx:
	$(CC) -S $(CC_FLAGS) $(AVX_FLAGS) ./simd/cpp/avx.cpp -o ./simd/cpp/avx.s
	c2goasm -a ./simd/cpp/avx.s ./simd/avx/AVX_amd64.s

compile_sse:
	$(CC) -S $(CC_FLAGS) $(SSE_FLAGS) ./simd/cpp/sse.cpp -o ./simd/cpp/sse.s
	c2goasm -a ./simd/cpp/sse.s ./simd/sse/SSE_amd64.s

compile_protos:
	protoc -I ./protobuf/proto --go_out=plugins=grpc:./protobuf ./protobuf/proto/*.proto

compile: compile_protos
	GOOS=linux GOARCH=amd64 go build -o ./bin/anndb-linux-amd64-$(VERSION) ./cmd/anndb/main.go
	GOOS=darwin GOARCH=amd64 go build -o ./bin/anndb-darwin-amd64-$(VERSION) ./cmd/anndb/main.go

compile_cli: compile_protos
	GOOS=linux GOARCH=amd64 go build -o ./bin/anndb-cli-linux-amd64-$(VERSION) ./cmd/cli/main.go
	GOOS=darwin GOARCH=amd64 go build -o ./bin/anndb-cli-darwin-amd64-$(VERSION) ./cmd/cli/main.go

run_clean: compile
	rm -rf ./data/${ID}
	./bin/anndb-darwin-amd64-$(VERSION) --port="600${ID}" --join="${JOIN}" --data-dir="./data/${ID}"

run_clean_no_join: compile
	rm -rf ./data/${ID}
	./bin/anndb-darwin-amd64-$(VERSION) --port="600${ID}" --join="false" --data-dir="./data/${ID}"

run: compile
	./bin/anndb-darwin-amd64-$(VERSION) --port="600${ID}" --join="${JOIN}" --data-dir="./data/${ID}"
