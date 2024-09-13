package cobra

import (
	"fmt"

	"github.com/sambly/exchangebot/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update",

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("update called")
		debugLog, _ := cmd.Flags().GetBool("debug-log")
		cfg.DebugLog = debugLog
		fmt.Printf("Updated 'debug-log' to: %v\n", viper.GetBool("debug-log"))

		logger.InitLogger(cfg.DebugLog, cfg.ProductionLog)

		fmt.Println(cfg.String())
	},
}

func init() {

	RootCmd.AddCommand(updateCmd)
}
