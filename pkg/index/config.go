package index

import (
    "github.com/marekgalovic/anndb/pkg/math";
)

var hnswSearchAlgorithmNames = [...]string {
    "Simple",
    "Heuristic",
}

type hnswSearchAlgorithm int

const (
    HnswSearchSimple hnswSearchAlgorithm = iota
    HnswSearchHeuristic
)

func (a hnswSearchAlgorithm) String() string {    
    return hnswSearchAlgorithmNames[a]
}

// Options
type HnswOption interface {
    apply(*hnswConfig)
}

type hnswOption struct {
    applyFunc func(*hnswConfig)
}

func (opt *hnswOption) apply(config *hnswConfig) {
    opt.applyFunc(config)
}

func HnswLevelMultiplier(value float32) HnswOption {
    return &hnswOption{func(config *hnswConfig) {
        config.levelMultiplier = value
    }}
}

func HnswEf(value int) HnswOption {
    return &hnswOption{func(config *hnswConfig) {
        config.ef = value
    }}
}

func HnswEfConstruction(value int) HnswOption {
    return &hnswOption{func(config *hnswConfig) {
        config.efConstruction = value
    }}
}

func HnswM(value int) HnswOption {
    return &hnswOption{func(config *hnswConfig) {
        config.m = value
    }}
}

func HnswMmax(value int) HnswOption {
    return &hnswOption{func(config *hnswConfig) {
        config.mMax = value
    }}
}

func HnswMmax0(value int) HnswOption {
    return &hnswOption{func(config *hnswConfig) {
        config.mMax0 = value
    }}
}

func HnswSearchAlgorithm(value hnswSearchAlgorithm) HnswOption {
    return &hnswOption{func(config *hnswConfig) {
        config.searchAlgorithm = value
    }}
}

type hnswConfig struct {
    searchAlgorithm hnswSearchAlgorithm
    levelMultiplier float32
    ef int
    efConstruction int
    m int
    mMax int
    mMax0 int
}

func newHnswConfig(options []HnswOption) hnswConfig {
	config := hnswConfig {
		searchAlgorithm: HnswSearchSimple,
        levelMultiplier: -1,
        ef: 20,
        efConstruction: 200,
        m: 16,
        mMax: -1,
        mMax0: -1,
	}
	for _, option := range options {
		option.apply(&config)
	}

	if config.levelMultiplier == -1 {
        config.levelMultiplier = 1.0 / math.Log(float32(config.m))
    }
    if config.mMax == -1 {
        config.mMax = config.m
    }
    if config.mMax0 == -1 {
        config.mMax0 = 2 * config.m
    }

    return config
}