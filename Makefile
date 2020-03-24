compile_protos:
	protoc -I ./proto --go_out=plugins=grpc:./pkg/protobuf ./proto/*.proto
	protoc -I ./proto/internal --go_out=plugins=grpc:./pkg/protobuf/internal ./proto/internal/*.proto
