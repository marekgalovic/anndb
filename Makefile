CC=clang
CC_FLAGS=-masm=intel -mno-red-zone -mstackrealign -mllvm -inline-threshold=1000 -fno-asynchronous-unwind-tables \
	-fno-exceptions -fno-rtti -fno-jump-tables -fno-builtin -fno-jump-tables -O3
AVX_FLAGS=-mavx
# SSE_FLAGS=

compile_avx:
	$(CC) -S $(CC_FLAGS) $(AVX_FLAGS) ./pkg/simd/cpp/avx.cpp -o ./pkg/simd/cpp/avx.s
	c2goasm -a ./pkg/simd/cpp/avx.s ./pkg/simd/avx/AVX_amd64.s

compile_sse:
	$(CC) -S $(CC_FLAGS) $(SSE_FLAGS) ./pkg/simd/cpp/sse.cpp -o ./pkg/simd/cpp/sse.s
	c2goasm -a ./pkg/simd/cpp/sse.s ./pkg/simd/sse/SSE_amd64.s

compile_protos:
	protoc -I ./proto --go_out=plugins=grpc:./pkg/protobuf ./proto/*.proto

run_clean:
	rm -rf ./data/${ID}
	mkdir -p ./data/${ID}
	go run cmd/anndb/main.go --port="600${ID}" --join="${JOIN}" --data-dir="./data/${ID}"

run:
	go run cmd/anndb/main.go --port="600${ID}" --join="${JOIN}" --data-dir="./data/${ID}"
