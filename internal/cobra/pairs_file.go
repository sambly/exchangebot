package cobra

import (
	"context"
	"fmt"
	"os"

	"github.com/sambly/exchangeService/pkg/exchange"
	"github.com/spf13/cobra"
)

var getPairsCmd = &cobra.Command{
	Use:   "pairs-to-file",
	Short: "pairs-to-file",
	Run: func(cmd *cobra.Command, args []string) {

		fmt.Println("get-pairs called")

		binance, err := exchange.NewBinance(context.Background())
		if err != nil {
			fmt.Printf("error create binance: %v", err)
			return
		}

		pairs, err := binance.GetPairsToUSDT()
		if err != nil {
			fmt.Printf("error get pairs frome binance: %v", err)
			return
		}

		// Открываем или создаем файл для записи
		file, err := os.Create("configs/pairs.txt")
		if err != nil {
			fmt.Printf("error creating file: %v\n", err)
			return
		}
		defer file.Close()

		// Записываем каждую пару на новой строке
		for _, pair := range pairs {
			_, err := file.WriteString(pair + "\n")
			if err != nil {
				fmt.Printf("error writing to file: %v\n", err)
				return
			}
		}

		fmt.Println("Pairs successfully written to pairs.txt")

	},
}

func init() {
	RootCmd.AddCommand(getPairsCmd)
}
