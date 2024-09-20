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

		viper.SetConfigFile(filename)

		if err := viper.ReadInConfig(); err != nil {
			fmt.Printf("Error reading config file: %v\n", err)
			return
		}
		if cmd.Flags().Changed("debug-log") {
			debugLog, _ := cmd.Flags().GetBool("debug-log")
			viper.Set("debug-log", debugLog)
			fmt.Printf("Updated debug-log to %v\n", debugLog)
		}

		if cmd.Flags().Changed("production-log") {
			productionLog, _ := cmd.Flags().GetBool("production-log")
			viper.Set("production-log", productionLog)
			fmt.Printf("Updated production-log to %v\n", productionLog)
		}

		if err := viper.WriteConfig(); err != nil {
			fmt.Printf("Error writing config file: %v", err)
		}

	},
}

func init() {

	updateCmd.Flags().Bool("debug-log", false, "Enable or disable debug logging")
	updateCmd.Flags().Bool("production-log", false, "Enable or disable production log format")

	RootCmd.AddCommand(updateCmd)
}
