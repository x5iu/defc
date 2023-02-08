package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/x5iu/defc/gen"
	"go/format"
	"gopkg.in/yaml.v3"
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
		gen.FeatureApiError,
		gen.FeatureApiNoRt,
		gen.FeatureSqlxIn,
		gen.FeatureSqlxLog,
		gen.FeatureSqlxRebind,
		gen.FeatureSqlxNoRt,
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

var (
	defc = &cobra.Command{
		Use:           "defc",
		Version:       "v1.11.2",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd:   true,
			DisableNoDescFlag:   true,
			DisableDescriptions: true,
			HiddenDefaultCmd:    true,
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				defer os.Exit(0)
				return cmd.Usage()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if err = checkFlags(); err != nil {
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

			builder := gen.NewCliBuilder(modeMap[mode]).
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

			return save(path.Join(pwd, output), buffer.Bytes())
		},
	}

	generate = &cobra.Command{
		Use:           "generate",
		Short:         "Generate code from schema file",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if err = checkFlags(); err != nil {
				return err
			}

			file := args[0]

			schema, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("os.ReadFile(%s): %w", args[0], err)
			}

			var cfg gen.Config
			switch ext := path.Ext(file); ext {
			case ".json":
				if err = json.Unmarshal(schema, &cfg); err != nil {
					return fmt.Errorf("json.Unmarshal: %w", err)
				}
			case ".toml":
				if err = toml.Unmarshal(schema, &cfg); err != nil {
					return fmt.Errorf("toml.Unmarshal: %w", err)
				}
			case ".yaml", ".yml":
				if err = yaml.Unmarshal(schema, &cfg); err != nil {
					return fmt.Errorf("yaml.Unmarshal: %w", err)
				}
			default:
				return fmt.Errorf("%s currently does not support schema extension %q", cmd.Root().Name(), ext)
			}

			cfg.Features = append(cfg.Features, features...)
			cfg.Imports = append(cfg.Imports, imports...)
			cfg.Funcs = append(cfg.Funcs, funcs...)

			var buffer bytes.Buffer
			if err = gen.Generate(&buffer, modeMap[mode], &cfg); err != nil {
				return err
			}

			return save(output, buffer.Bytes())
		},
	}
)

func save(name string, code []byte) error {
	fmtCode, err := format.Source(code)
	if err != nil {
		return fmt.Errorf("format.Source: \n\n%s\n\n%w", code, err)
	}

	if err = os.WriteFile(name, fmtCode, 0644); err != nil {
		return fmt.Errorf("os.WriteFile(%q, 0644): %w", name, err)
	}

	return nil
}

func checkFlags() (err error) {
	if len(mode) == 0 {
		return fmt.Errorf("`-m/--mode` required, available modes are: [%s]", printStrings(validModes))
	}
	if len(output) == 0 {
		return fmt.Errorf("`-o/--output` required")
	}
	if genMode := modeMap[mode]; !genMode.IsValid() {
		return fmt.Errorf("invalid mode %q, available modes are: [%s]", mode, printStrings(validModes))
	}
	if err = checkFeatures(features); err != nil {
		return err
	}
	return nil
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
	defc.AddCommand(generate)
	defc.SetHelpCommand(&cobra.Command{Hidden: true})

	defc.PersistentFlags().StringVarP(&mode, "mode", "m", "", fmt.Sprintf("mode=[%s]", printStrings(validModes)))
	defc.PersistentFlags().StringVarP(&output, "output", "o", "", "output file name")
	defc.PersistentFlags().StringSliceVarP(&features, "features", "f", nil, fmt.Sprintf("features=[%s]", printStrings(validFeatures)))
	defc.PersistentFlags().StringArrayVar(&imports, "import", nil, "additional imports")
	defc.PersistentFlags().StringArrayVar(&funcs, "func", nil, "additional funcs")
	defc.MarkPersistentFlagRequired("mode")
	defc.MarkPersistentFlagRequired("output")
}

func main() {
	cobra.CheckErr(defc.Execute())
}
