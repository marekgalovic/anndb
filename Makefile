compile_protos:
	protoc -I ./proto --go_out=plugins=grpc:./pkg/protobuf ./proto/*.proto
