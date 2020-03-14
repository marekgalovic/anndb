package main

import (
	"github.com/marekgalovic/anndb/pkg/index";
	"github.com/marekgalovic/anndb/pkg/math";

	log "github.com/Sirupsen/logrus";
)

func main() {
	i := index.NewHnsw(32, index.NewEuclideanSpace());

	for id := 0; id < 10; id++ {
		log.Info(i.Insert(uint64(id), math.RandomUniformVector(32)));	
	}
	
	log.Info(i.Get(1));
}	