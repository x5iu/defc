package main

import (
	"bytes"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/x5iu/defc/gen"
	"go/format"
	"os"
	"path"
	"strconv"
)

const (
	EnvPWD       = "PWD"
	EnvGoPackage = "GOPACKAGE"
	EnvGoFile    = "GOFILE"
	EnvGoLine    = "GOLINE"
)

var (
	modeMap       map[string]gen.Mode
	validModes    []string
	validFeatures = []string{
		gen.FeatureApiCache,
		gen.FeatureApiLog,
		gen.FeatureApiClient,
		gen.FeatureApiPage,
		gen.FeatureSqlxLog,
		gen.FeatureSqlxRebind,
	}
)

func init() {
	modeMap = make(map[string]gen.Mode, gen.ModeEnd-gen.ModeStart-1)
	validModes = make([]string, 0, gen.ModeEnd-gen.ModeStart-1)
	for m := gen.ModeStart + 1; m < gen.ModeEnd; m++ {
		modeMap[m.String()] = m
		validModes = append(validModes, m.String())
	}
}

var (
	mode     string
	output   string
	features []string
	imports  []string
	funcs    []string
)

var defc = &cobra.Command{
	Use:     "defc",
	Version: "v1.1.1",
	Args: func(cmd *cobra.Command, args []string) error {
		switch modeMap[mode] {
		case gen.ModeApi, gen.ModeSqlx:
			return cobra.NoArgs(cmd, args)
		default:
			return fmt.Errorf("invalid mode %q", mode)
		}
	},
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		if err = checkFeatures(features); err != nil {
			return err
		}

		var (
			pwd  = os.Getenv(EnvPWD)
			file = os.Getenv(EnvGoFile)
			doc  []byte
			pos  int
		)

		if doc, err = os.ReadFile(path.Join(pwd, file)); err != nil {
			return fmt.Errorf("$GOFILE: os.ReadFile(%q): %w", path.Join(pwd, file), err)
		}

		if pos, err = strconv.Atoi(os.Getenv(EnvGoLine)); err != nil {
			return fmt.Errorf("$GOLINE: strconv.Atoi(%s): %w", os.Getenv(EnvGoLine), err)
		}

		builder := gen.NewBuilder(modeMap[mode]).
			WithFeats(features).
			WithImports(imports).
			WithFuncs(funcs).
			WithPkg(os.Getenv(EnvGoPackage)).
			WithPwd(pwd).
			WithFile(file, doc).
			WithPos(pos)

		var buffer bytes.Buffer
		if err = builder.Build(&buffer); err != nil {
			return err
		}

		fmtCode, err := format.Source(buffer.Bytes())
		if err != nil {
			return fmt.Errorf("format.Source: \n\n%s\n\n%w", buffer.Bytes(), err)
		}

		if err = os.WriteFile(path.Join(pwd, output), fmtCode, 0644); err != nil {
			return fmt.Errorf("os.WriteFile(%s, 0644): %w", path.Join(pwd, output), err)
		}

		return nil
	},
}

func checkFeatures(features []string) error {
	if len(features) == 0 {
		return nil
	}

Check:
	for _, feature := range features {
		for _, valid := range validFeatures {
			if feature == valid {
				continue Check
			}
		}

		return fmt.Errorf("checkFeatures: invalid feature %s, available features are: \n\n%s\n\n",
			strconv.Quote(feature),
			printStrings(validFeatures))
	}

	return nil
}

func printStrings(strings []string) string {
	var buf bytes.Buffer
	for i, s := range strings {
		buf.WriteString(strconv.Quote(s))
		if i < len(strings)-1 {
			buf.WriteString(", ")
		}
	}
	return buf.String()
}

func init() {
	defc.Flags().StringVarP(&mode, "mode", "m", "", fmt.Sprintf("mode=[%s]", printStrings(validModes)))
	defc.Flags().StringVarP(&output, "output", "o", "", "output file name")
	defc.Flags().StringSliceVarP(&features, "features", "f", nil, fmt.Sprintf("features=[%s]", printStrings(validFeatures)))
	defc.Flags().StringArrayVar(&imports, "import", nil, "additional imports")
	defc.Flags().StringArrayVar(&funcs, "func", nil, "additional funcs")
	defc.MarkFlagRequired("mode")
	defc.MarkFlagRequired("output")
}

func main() {
	cobra.CheckErr(defc.Execute())
}
