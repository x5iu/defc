package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/x5iu/defc/gen/lint"
)

var (
	lintStrict     bool
	lintOnly       []string
	lintIgnore     []string
	lintFormat     string
	lintFailOn     string
	lintSafePragma = "//go:lint-safe"
)

var lintCmd = &cobra.Command{
	Use:   "lint [PATH...]",
	Short: "Context-aware linter for defc schema files",
	Long: `The lint subcommand scans Go files (or directories) containing defc schema
interfaces and reports unsafe template interpolations in SQL, URL, and HTTP
header contexts.

Exit codes:
  0   success, no findings (or all below --fail-on)
  2   findings at or above --fail-on threshold
  64  usage / IO error`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := lint.Options{
			Only:   lintOnly,
			Strict: lintStrict,
		}
		for _, pat := range lintIgnore {
			re, err := regexp.Compile(pat)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --ignore pattern %q: %w", pat, err))
			}
			opts.Ignore = append(opts.Ignore, re)
		}
		rep, err := lint.Run(args, opts)
		if err != nil {
			return usageErr(err)
		}
		switch lintFormat {
		case "", "text":
			lint.EmitText(os.Stdout, rep)
		case "json":
			if err := lint.EmitJSON(os.Stdout, rep); err != nil {
				return usageErr(err)
			}
		default:
			return usageErr(fmt.Errorf("unknown --format %q (want text|json)", lintFormat))
		}
		threshold := lintFailOn
		if threshold == "" {
			threshold = "blocker"
		}
		switch threshold {
		case "none":
			return nil
		case "note":
			if rep.Summary.Blockers+rep.Summary.Notes > 0 {
				os.Exit(2)
			}
		case "blocker":
			if rep.Summary.Blockers > 0 {
				os.Exit(2)
			}
		default:
			return usageErr(fmt.Errorf("unknown --fail-on %q (want blocker|note|none)", lintFailOn))
		}
		return nil
	},
}

// usageErr wraps an error so that main exits with status 64 for
// usage/IO failures.
type usageError struct{ err error }

func (u *usageError) Error() string { return u.err.Error() }
func usageErr(err error) error      { return &usageError{err: err} }

func init() {
	defc.AddCommand(lintCmd)
	f := lintCmd.Flags()
	f.BoolVar(&lintStrict, "strict", false, "promote NOTE findings to BLOCKER")
	f.StringSliceVar(&lintOnly, "only", nil, "restrict to contexts (sql, url, header)")
	f.StringSliceVar(&lintIgnore, "ignore", nil, "file path regexes to skip")
	f.StringVar(&lintFormat, "format", "text", "output format: text or json")
	f.StringVar(&lintFailOn, "fail-on", "blocker", "exit 2 if findings at threshold: blocker, note, none")
}
