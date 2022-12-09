package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"strconv"
)

const (
	EnvPWD       = "PWD"
	EnvGoPackage = "GOPACKAGE"
	EnvGoFile    = "GOFILE"
	EnvGoLine    = "GOLINE"

	ModeApi  = "api"
	ModeSqlx = "sqlx"

	FileMode = 0644
)

var (
	PackageName = os.Getenv(EnvGoPackage)
	CurrentDir  = os.Getenv(EnvPWD)
	CurrentFile = os.Getenv(EnvGoFile)
	LineNum, _  = strconv.Atoi(os.Getenv(EnvGoLine))

	FileContent []byte
)

var (
	mode     string
	features []string
	output   string
)

var defc = &cobra.Command{
	Use:     "defc",
	Version: "v1.0.1",
	Args: func(cmd *cobra.Command, args []string) error {
		switch mode {
		case ModeApi, ModeSqlx:
			return cobra.NoArgs(cmd, args)
		default:
			return nil
		}
	},
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkFeatures(features); err != nil {
			return err
		}

		switch mode {
		case ModeApi:
			return genApi(cmd, args)
		case ModeSqlx:
			return genSqlx(cmd, args)
		default:
			return nil
		}
	},
}

var validFeatures = []string{
	FeatureApiCache,
	FeatureApiLog,
	FeatureApiClient,
	FeatureSqlxLog,
	FeatureSqlxRebind,
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
			quote(feature),
			printStrings(validFeatures))
	}

	return nil
}

func init() {
	defc.Flags().StringVarP(&mode, "mode", "m", "", "mode=[\"api\", \"sqlx\"]")
	defc.Flags().StringSliceVarP(&features, "features", "f", nil, fmt.Sprintf("features=[%s]", printStrings(validFeatures)))
	defc.Flags().StringVarP(&output, "output", "o", "", "output file name")
}

func init() {
	if CurrentFile != "" {
		var err error
		FileContent, err = read(join(CurrentDir, CurrentFile))
		cobra.CheckErr(err)
	}
}

func main() {
	cobra.CheckErr(defc.Execute())
}
