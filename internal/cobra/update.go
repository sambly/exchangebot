package cobra

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update configuration settings",
	Run: func(cmd *cobra.Command, args []string) {

		fmt.Println("update called")

		viper.SetConfigFile(filenameConfigReload)

		if err := viper.ReadInConfig(); err != nil {
			fmt.Printf("Error reading config file: %v\n", err)
			return
		}
		if cmd.Flags().Changed("log-debug") {
			debugLog, _ := cmd.Flags().GetBool("log-debug")
			viper.Set("log.debug", debugLog)
			fmt.Printf("Updated log-debug to %v\n", debugLog)
		}

		if cmd.Flags().Changed("log-production") {
			productionLog, _ := cmd.Flags().GetBool("log-production")
			viper.Set("log.production", productionLog)
			fmt.Printf("Updated log-production to %v\n", productionLog)
		}

		if err := viper.WriteConfigAs(filenameConfigReload); err != nil {
			fmt.Printf("Error writing config file: %v", err)
		}

	},
}

func init() {

	updateCmd.Flags().Bool("log-debug", false, "Enable or disable debug logging")
	updateCmd.Flags().Bool("log-production", false, "Enable or disable production log format")

	RootCmd.AddCommand(updateCmd)
}
