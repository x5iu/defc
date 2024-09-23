//go:build future
// +build future

package main

import (
	"github.com/spf13/cobra"
	"github.com/x5iu/defc/gen"
)

func init() {
	cobra.OnInitialize(func() {
		features = append(features, gen.FeatureApiFuture, gen.FeatureSqlxFuture)
	})
}
