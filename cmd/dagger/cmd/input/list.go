package input

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"dagger.io/go/cmd/dagger/cmd/common"
	"dagger.io/go/cmd/dagger/logger"
	"dagger.io/go/dagger"
	"dagger.io/go/dagger/compiler"
	"dagger.io/go/dagger/state"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listCmd = &cobra.Command{
	Use:   "list [TARGET] [flags]",
	Short: "List for the inputs of an environment",
	Args:  cobra.MaximumNArgs(1),
	PreRun: func(cmd *cobra.Command, args []string) {
		// Fix Viper bug for duplicate flags:
		// https://github.com/spf13/viper/issues/233
		if err := viper.BindPFlags(cmd.Flags()); err != nil {
			panic(err)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		lg := logger.New()
		ctx := lg.WithContext(cmd.Context())

		environment := common.GetCurrentEnvironmentState(ctx)

		lg = lg.With().
			Str("environment", environment.Name).
			Logger()

		c, err := dagger.NewClient(ctx, "", false)
		if err != nil {
			lg.Fatal().Err(err).Msg("unable to create client")
		}

		_, err = c.Do(ctx, environment, func(lCtx context.Context, lDeploy *dagger.Environment, lSolver dagger.Solver) error {
			inputs := lDeploy.ScanInputs(ctx)

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "Input\tType\tValue\tSet by user")

			for _, inp := range inputs {
				isConcrete := (inp.IsConcreteR() == nil)
				_, hasDefault := inp.Default()
				valStr := "-"
				if isConcrete {
					valStr, _ = inp.Cue().String()
				}
				if hasDefault {
					valStr = fmt.Sprintf("%s (default)", valStr)
				}

				if !viper.GetBool("all") {
					// skip input that is not overridable
					if !hasDefault && isConcrete {
						continue
					}
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%t\n",
					inp.Path(),
					getType(inp),
					valStr,
					isUserSet(environment, inp),
				)
			}

			w.Flush()
			return nil
		})

		if err != nil {
			lg.Fatal().Err(err).Msg("failed to query environment")
		}

	},
}

func isUserSet(env *state.State, val *compiler.Value) bool {
	for key := range env.Inputs {
		if val.Path().String() == key {
			return true
		}
	}

	return false
}

func getType(val *compiler.Value) string {
	if val.HasAttr("artifact") {
		return "dagger.#Artifact"
	}
	if val.HasAttr("secret") {
		return "dagger.#Secret"
	}
	return val.Cue().IncompleteKind().String()
}

func init() {
	listCmd.Flags().BoolP("all", "a", false, "List all inputs (include non-overridable)")

	if err := viper.BindPFlags(listCmd.Flags()); err != nil {
		panic(err)
	}
}
