// Command poolalias runs the poolalias analyzer as a standalone go vet
// vettool. See the poolalias package for rule descriptions.
package main

import (
	"github.com/x5iu/defc/gen/analyzer/poolalias"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(poolalias.Analyzer)
}
