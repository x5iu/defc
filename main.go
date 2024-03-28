package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/x5iu/defc/gen"
	runtime "github.com/x5iu/defc/runtime"
	"go/format"
	goimport "golang.org/x/tools/imports"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"path/filepath"
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
		gen.FeatureApiLogx,
		gen.FeatureApiClient,
		gen.FeatureApiPage,
		gen.FeatureApiError,
		gen.FeatureApiNoRt,
		gen.FeatureApiFuture,
		gen.FeatureSqlxIn,
		gen.FeatureSqlxLog,
		gen.FeatureSqlxRebind,
		gen.FeatureSqlxNoRt,
		gen.FeatureSqlxFuture,
		gen.FeatureSqlxCallback,
		gen.FeatureSqlxAnyCallback,
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
	mode              string
	output            string
	features          []string
	imports           []string
	disableAutoImport bool
	funcs             []string
)

var (
	defc = &cobra.Command{
		Use:   "defc --mode MODE --output FILE [--features LIST] [--import PACKAGE]... [--func FUNCTION]...",
		Short: "By defining the Schema, use go generate to generate database CRUD or HTTP request code.",
		Long: `defc originates from the tedium of repetitively writing code for "create, read, update, delete" (CRUD) 
operations and "network interface integration" in our daily work and life.

For example, for database queries, we often need to:

1. Define a new function or method;
2. Write a new SQL query;
3. Execute the query, handle errors, and map the results to a structure;
4. If there are multiple SQL statements, initiate a transaction, and perform commit or rollback;
5. Log the query;
6. ...

Similarly, for network interface integration, for a new interface, we often:

1. Define a new function or method;
2. Set the interface URL, configure parameters (such as Headers, Query, Body in HTTP requests);
3. Make the request, handle errors, and map the response to a structure;
4. If it involves pagination, concatenate the results of multiple paginated queries into the final result;
5. Log the request;
6. ...

All of the above are repeated several times when writing new requirements or scenarios. Especially the parts related 
to queries, requests, error handling, transaction commit/rollback, data mapping, list concatenation, and log recording, 
which are all logically identical repetitive codes. Writing them is very annoying; some codes are very long, and 
copying and pasting require various changes to variable names, method names, and configuration information, which 
greatly affects development efficiency;

Unfortunately, the Go language does not provide official macro features, and we cannot use macros to complete these 
complex repetitive codes like Rust does (of course, macros also have their limitations; they are devastating to code 
readability when not expanded and also affect IDE completion). However, fortunately, Go provides a workaround with go 
generate. Through go generate, we can approximately provide macro functionality, that is, code generation capabilities.

Based on the above background, I wanted to implement a code generation tool. By defining the Schema of a query or 
request, it is possible to automatically generate code for the related CRUD operations or HTTP requests, which includes 
parameter construction, error handling, result mapping, and log recording logic. defc is my experimental attempt at 
such a schema-to-code generation; "def" stands for "define," indicating the behavior of setting up a Schema. Currently, 
defc provides the following two scenarios of code generation features:

* CRUD code generation based on sqlx for databases
* HTTP interface request code generation based on the net/http package in the Golang standard library`,
		Version:       runtime.Version,
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
				return fmt.Errorf("$GOFILE: os.ReadFile(%q): %w", filepath.Join(pwd, file), err)
			}

			if pos, err = strconv.Atoi(os.Getenv(EnvGoLine)); err != nil {
				return fmt.Errorf("$GOLINE: strconv.Atoi(%s): %w", os.Getenv(EnvGoLine), err)
			}

			builder := gen.NewCliBuilder(modeMap[mode]).
				WithFeats(features).
				// Since we are using the golang.org/x/tools/imports
				// package to handle imports, there is no need to
				// use the auto-import feature.
				//
				// disableAutoImport = true
				WithImports(imports, true).
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
			switch ext := filepath.Ext(file); ext {
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

func save(name string, code []byte) (err error) {
	code, err = format.Source(code)
	if err != nil {
		return fmt.Errorf("format.Source: \n\n%s\n\n%w", code, err)
	}

	if err = os.WriteFile(name, code, 0644); err != nil {
		return fmt.Errorf("os.WriteFile(%q, 0644): %w", name, err)
	}

	if !disableAutoImport {
		code, err = goimport.Process(name, code, nil)
		if err != nil {
			return fmt.Errorf("imports.Process: \n\n%s\n\n%w", code, err)
		}

		if err = os.WriteFile(name, code, 0644); err != nil {
			return fmt.Errorf("os.WriteFile(%q, 0644): %w", name, err)
		}
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
	defc.PersistentFlags().BoolVar(&disableAutoImport, "disable-auto-import", false, "disable auto import and import packages manually by '--import' option")
	defc.PersistentFlags().StringArrayVar(&funcs, "func", nil, "additional funcs")
	defc.MarkPersistentFlagRequired("mode")
	defc.MarkPersistentFlagRequired("output")
}

func main() {
	cobra.CheckErr(defc.Execute())
}
