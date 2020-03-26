CC=clang
CC_FLAGS=-masm=intel -mno-red-zone -mstackrealign -mllvm -inline-threshold=1000 -fno-asynchronous-unwind-tables \
	-fno-exceptions -fno-rtti -fno-jump-tables -fno-builtin -fno-jump-tables -O3
AVX2_FLAGS=-mavx2

compile_avx2:
	$(CC) -S $(CC_FLAGS) $(AVX2_FLAGS) ./pkg/simd/cpp/avx2.cpp -o ./pkg/simd/cpp/avx2.s
	c2goasm -a ./pkg/simd/cpp/avx2.s ./pkg/simd/avx2/AVX2_amd64.s

compile_protos:
	protoc -I ./proto --go_out=plugins=grpc:./pkg/protobuf ./proto/*.proto
