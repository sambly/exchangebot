package application

import (
	"context"
	"time"

	"github.com/sambly/exchangebot/internal/logger"
)

// ВРЕМЕННАЯ диагностика (см. ARCHITECTURE.md §10): раз в diagIndicatorInterval
// пишет в лог сырые значения индикаторов таблицы "Потенциал" (OFI/VPIN/
// Уровни/Режим, плюс старый снапшот-имбаланс для сравнения скорости) по
// нескольким парам - чтобы вручную оценить, как часто и насколько сильно они
// реально меняются на практике, и является ли это нормальным поведением.
// Не постоянная часть системы - убрать файл целиком и строчку в Run(), когда
// логов накопится достаточно для анализа.
var diagIndicatorPairs = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "DOGEUSDT", "PEPEUSDT"}

const (
	// diagIndicatorPeriod - для Уровней/Режима (они зависят от периода, в
	// отличие от OFI/VPIN/имбаланса, которые общие на пару).
	diagIndicatorPeriod = "15m"
	// diagIndicatorInterval - тот же порядок, что у ofiWindowDuration (10с) -
	// чаще смысла нет: быстрее OFI всё равно не коммитится.
	diagIndicatorInterval = 10 * time.Second
)

var diagLogger = logger.AddFieldsEmpty()

// runIndicatorDiag регистрируется в errgroup Run() и живёт, пока жив gCtx -
// останавливается штатно вместе со всем приложением, отдельного графа
// завершения не требует.
func (app *Application) runIndicatorDiag(ctx context.Context) error {
	ticker := time.NewTicker(diagIndicatorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			app.logIndicatorSnapshot()
		}
	}
}

func (app *Application) logIndicatorSnapshot() {
	// GetAllPriceLevels, а не GetPriceLevels(pair, period) в цикле по парам:
	// у второго свой поход в БД на КАЖДЫЙ вызов (см. комментарий у
	// entrysetup.GetAllStrengthComponents) - в цикле по 5 парам это 5 лишних
	// походов в БД каждые 10 секунд. GetAllPriceLevels кэширован на 30с.
	allLevels := app.AssetsSetup.GetAllPriceLevels()

	for _, pair := range diagIndicatorPairs {
		ofiVal, ofiZ, ofiReady := app.AssetsDepth.GetOFIZScore(pair)
		ofiSide, ofiConfirmed := app.AssetsDepth.GetOFIConfirmedSide(pair)

		imbVal, imbZ, imbReady := app.AssetsDepth.GetImbalanceZScore(pair)

		vpinVal, vpinZ, vpinReady := app.AssetsPrices.GetVPINZScore(pair)
		vpinSide, vpinConfirmed := app.AssetsPrices.GetVPINConfirmedSide(pair)

		regime, regimeReady := app.AssetsSetup.GetVolatilityRegime(pair, diagIndicatorPeriod)

		levels, levelsReady := allLevels[pair][diagIndicatorPeriod]

		microPrice, microDeviationPercent, microReady := app.AssetsDepth.GetMicroPrice(pair)

		rsValue, rsZ, rsReady := app.AssetsPrices.GetRelativeStrengthZScore(pair, diagIndicatorPeriod)
		rsSide, rsConfirmed := app.AssetsPrices.GetRelativeStrengthConfirmedSide(pair, diagIndicatorPeriod)

		diagLogger.Infof(
			"diag pair=%s | ofi val=%.6g z=%.2f ready=%v side=%s confirmed=%v | imbalance val=%.4f z=%.2f ready=%v | vpin val=%.4f z=%.2f ready=%v side=%s confirmed=%v | regime=%.2f ready=%v | levels support%%=%.2f resistance%%=%.2f hasSupport=%v hasResistance=%v ready=%v | micro price=%.8g dev%%=%.4f ready=%v | relstrength val=%.4f z=%.2f ready=%v side=%s confirmed=%v",
			pair,
			ofiVal, ofiZ, ofiReady, ofiSide, ofiConfirmed,
			imbVal, imbZ, imbReady,
			vpinVal, vpinZ, vpinReady, vpinSide, vpinConfirmed,
			regime, regimeReady,
			levels.Support.DistancePercent, levels.Resistance.DistancePercent, levels.HasSupport, levels.HasResistance, levelsReady,
			microPrice, microDeviationPercent, microReady,
			rsValue, rsZ, rsReady, rsSide, rsConfirmed,
		)
	}
}
