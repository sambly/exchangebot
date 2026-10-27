package potential

import (
	"math"

	"github.com/sambly/exchangebot/internal/entrysetup"
	"github.com/sambly/exchangebot/internal/order"
)

// contributionClamp - потолок |вклада| одного компонента в композит. Портировано
// из frontend/src/components/DataPanel/DataStrength.vue (buildRow) - см. Go-
// комментарий там же (ARCHITECTURE.md §10.5): OFI на настоящих (не вырожденных)
// всплесках честно даёт z в сотни, и без потолка один такой тик полностью
// перевешивает средний композит по остальным компонентам.
const contributionClamp = 10.0

func clampContribution(value float64) float64 {
	if value > contributionClamp {
		return contributionClamp
	}
	if value < -contributionClamp {
		return -contributionClamp
	}
	return value
}

// ratioContribution - см. ratioContribution в DataStrength.vue: LevelsScore
// (>=1, без верхней границы) переводится логарифмом в шкалу, сравнимую с
// z-score остальных компонентов.
func ratioContribution(side order.SideType, score float64) float64 {
	sign := 1.0
	if side == order.SideTypeSell {
		sign = -1.0
	}
	if score < 1 {
		score = 1
	}
	return sign * math.Log(score)
}

func signOf(v float64) float64 {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

// compositeFromComponents - свод entrysetup.StrengthComponents в одно число,
// портировано из buildRow (DataStrength.vue) с той же логикой:
//   - OFI и RelativeStrength голосуют только при подтверждённой стороне
//     (Confirmed) - см. Go-комментарий у StrengthComponents.OFIConfirmed;
//   - VPIN голосует БЕЗ confirm-гейта (ARCHITECTURE.md §10.10 - streak там не
//     набирается практически никогда, бакет и без того интеграл ~20 минут);
//   - Levels голосует всегда, через ratioContribution;
//   - Regime - множитель на итог, а не голос (сжатие волатильности усиливает
//     то, что уже показали голосующие компоненты).
//
// agree - сколько из проголосовавших компонентов совпало по знаку с итоговым
// composite (тот же agree/total, что в таблице "Потенциал" во фронте).
func compositeFromComponents(c entrysetup.StrengthComponents, enabled ComponentsConfig) (composite float64, agree, total int) {
	var contributions []float64

	if enabled.OFI && c.HasOFI && c.OFIConfirmed {
		contributions = append(contributions, clampContribution(c.OFIZScore))
	}
	if enabled.VPIN && c.HasVPIN {
		contributions = append(contributions, clampContribution(c.VPINZScore))
	}
	if enabled.RelativeStrength && c.HasRelativeStrength && c.RelativeStrengthConfirmed {
		contributions = append(contributions, clampContribution(c.RelativeStrengthZScore))
	}
	if enabled.Levels && c.HasLevels {
		contributions = append(contributions, clampContribution(ratioContribution(c.LevelsSide, c.LevelsScore)))
	}

	if len(contributions) == 0 {
		return 0, 0, 0
	}

	sum := 0.0
	for _, v := range contributions {
		sum += v
	}
	composite = sum / float64(len(contributions))

	if enabled.Regime && c.HasRegime && c.Regime < 1 && composite != 0 {
		boost := 2 - c.Regime
		if boost > 1.5 {
			boost = 1.5
		}
		composite *= boost
	}

	compositeSign := signOf(composite)
	for _, v := range contributions {
		if signOf(v) == compositeSign {
			agree++
		}
	}

	return composite, agree, len(contributions)
}

// levelFromComposite - тот же принцип, что у anomaly.LevelFromZScore, только
// порог применяется к |composite|, а не к z-score одной метрики.
func levelFromComposite(composite float64, t ThresholdsConfig) int {
	absComposite := math.Abs(composite)
	if absComposite >= t.Level3 {
		return 3
	}
	if absComposite >= t.Level2 {
		return 2
	}
	if absComposite >= t.Level1 {
		return 1
	}
	return 0
}
