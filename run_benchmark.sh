#!/bin/bash

Ms=(4 8 12 16 24 36 48 64 96)

for fname in "$@"
do
	for m in "${Ms[@]}"
	do
		echo "Dataset: $fname, M: $m"
		go run ./cmd/benchmark/ann-benchmark/main.go --file "$fname" --hnsw-m "$m"
	done
done