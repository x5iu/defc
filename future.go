package main

import (
	"github.com/x5iu/defc/gen"
)

func onInitialize() {
	features = append(features, gen.FeatureApiFuture, gen.FeatureSqlxFuture)
}
