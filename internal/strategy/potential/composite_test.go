package potential

import (
	"math"
	"testing"

	"github.com/sambly/exchangebot/internal/entrysetup"
	"github.com/sambly/exchangebot/internal/order"
)

var allComponentsEnabled = ComponentsConfig{
	OFI:              true,
	VPIN:             true,
	Levels:           true,
	RelativeStrength: true,
	Regime:           true,
}

// Без единого проголосовавшего компонента (все Has*=false, либо все
// требующие Confirmed не подтверждены) composite должен быть честным нулём,
// а не "случайно" не нулём из-за забытого continue где-то в своде.
func TestCompositeFromComponentsNoVotesReturnsZero(t *testing.T) {
	c := entrysetup.StrengthComponents{}

	composite, agree, total := compositeFromComponents(c, allComponentsEnabled)

	if composite != 0 || agree != 0 || total != 0 {
		t.Fatalf("ожидались нулевые composite/agree/total, получено composite=%.4f agree=%d total=%d", composite, agree, total)
	}
}

// OFI и Levels согласны по стороне (оба BUY) - composite должен быть средним
// их клипованных вкладов, agree=2/2.
func TestCompositeFromComponentsAveragesConfirmedOFIAndLevels(t *testing.T) {
	c := entrysetup.StrengthComponents{
		HasOFI:       true,
		OFIZScore:    3.0,
		OFIConfirmed: true,
		HasLevels:    true,
		LevelsSide:   order.SideTypeBuy,
		LevelsScore:  math.E, // log(e) = 1 - удобное число для проверки
	}

	composite, agree, total := compositeFromComponents(c, allComponentsEnabled)

	// Вклады: OFI=3.0, Levels=ratioContribution(BUY, e)=+1*log(e)=1.0
	want := (3.0 + 1.0) / 2
	if math.Abs(composite-want) > 1e-9 {
		t.Fatalf("composite = %.6f, ожидалось %.6f", composite, want)
	}
	if agree != 2 || total != 2 {
		t.Fatalf("agree/total = %d/%d, ожидалось 2/2", agree, total)
	}
}

// OFI без подтверждения (OFIConfirmed=false) не должен голосовать вовсе - это
// сама причина, по которой в композите есть Confirmed-гейт: секундный дёрг
// стороны не должен становиться частью решения.
func TestCompositeFromComponentsSkipsUnconfirmedOFI(t *testing.T) {
	c := entrysetup.StrengthComponents{
		HasOFI:       true,
		OFIZScore:    9.0,
		OFIConfirmed: false,
		HasLevels:    true,
		LevelsSide:   order.SideTypeBuy,
		LevelsScore:  math.E,
	}

	composite, agree, total := compositeFromComponents(c, allComponentsEnabled)

	if total != 1 {
		t.Fatalf("total = %d, ожидался 1 (только Levels - OFI не подтверждён)", total)
	}
	want := 1.0 // только вклад Levels
	if math.Abs(composite-want) > 1e-9 {
		t.Fatalf("composite = %.6f, ожидалось %.6f", composite, want)
	}
	if agree != 1 {
		t.Fatalf("agree = %d, ожидался 1", agree)
	}
}

// VPIN голосует БЕЗ confirm-гейта (см. ARCHITECTURE.md §10.10 - streak там
// практически никогда не набирается) - в отличие от OFI/RelativeStrength.
func TestCompositeFromComponentsVPINVotesWithoutConfirm(t *testing.T) {
	c := entrysetup.StrengthComponents{
		HasVPIN:       true,
		VPINZScore:    1.5,
		VPINConfirmed: false,
	}

	composite, agree, total := compositeFromComponents(c, allComponentsEnabled)

	if total != 1 {
		t.Fatalf("total = %d, ожидался 1 (VPIN должен голосовать без Confirmed)", total)
	}
	if math.Abs(composite-1.5) > 1e-9 {
		t.Fatalf("composite = %.6f, ожидалось 1.5", composite)
	}
	if agree != 1 {
		t.Fatalf("agree = %d, ожидался 1", agree)
	}
}

// Огромный выброс одного компонента (см. ARCHITECTURE.md §10.5/§10.10 -
// настоящие хвосты OFI в сотни) не должен один определять composite -
// contributionClamp должен срезать его вклад ДО усреднения.
func TestCompositeFromComponentsClampsOutlier(t *testing.T) {
	c := entrysetup.StrengthComponents{
		HasOFI:       true,
		OFIZScore:    434.9, // см. PEPEUSDT в §10.10
		OFIConfirmed: true,
		HasVPIN:      true,
		VPINZScore:   0.5,
	}

	composite, _, total := compositeFromComponents(c, allComponentsEnabled)

	if total != 2 {
		t.Fatalf("total = %d, ожидался 2", total)
	}
	want := (contributionClamp + 0.5) / 2
	if math.Abs(composite-want) > 1e-9 {
		t.Fatalf("composite = %.6f, ожидалось %.6f (клип OFI до %.0f)", composite, want, contributionClamp)
	}
}

// Отдельный компонент, отключённый в конфиге (enabled.OFI=false), не должен
// голосовать, даже если Has/Confirmed=true - тот же смысл, что у чекбоксов
// "Потенциала" во фронте.
func TestCompositeFromComponentsRespectsDisabledComponent(t *testing.T) {
	c := entrysetup.StrengthComponents{
		HasOFI:       true,
		OFIZScore:    5.0,
		OFIConfirmed: true,
	}
	disabled := allComponentsEnabled
	disabled.OFI = false

	composite, _, total := compositeFromComponents(c, disabled)

	if total != 0 || composite != 0 {
		t.Fatalf("отключённый компонент не должен голосовать, получено composite=%.4f total=%d", composite, total)
	}
}

// Regime<1 (сжатие волатильности) усиливает уже сформированный composite -
// множитель, не голос: сам по себе стороны не задаёт (проверяется отдельным
// тестом ниже), но домножает то, что показали голосующие компоненты.
func TestCompositeFromComponentsRegimeBoostsMagnitude(t *testing.T) {
	c := entrysetup.StrengthComponents{
		HasOFI:       true,
		OFIZScore:    2.0,
		OFIConfirmed: true,
		HasRegime:    true,
		Regime:       0.5, // boost = min(2-0.5, 1.5) = 1.5
	}

	composite, _, _ := compositeFromComponents(c, allComponentsEnabled)

	want := 2.0 * 1.5
	if math.Abs(composite-want) > 1e-9 {
		t.Fatalf("composite = %.6f, ожидалось %.6f (2.0 * boost 1.5)", composite, want)
	}
}

// Regime не должен паниковать/делить на ноль, если голосующих компонентов
// нет вовсе - composite остаётся 0, а не NaN/Inf от домножения нуля.
func TestCompositeFromComponentsRegimeSkippedWithoutVotes(t *testing.T) {
	c := entrysetup.StrengthComponents{
		HasRegime: true,
		Regime:    0.3,
	}

	composite, agree, total := compositeFromComponents(c, allComponentsEnabled)

	if composite != 0 || agree != 0 || total != 0 {
		t.Fatalf("без голосов regime не должен ничего менять, получено composite=%.4f agree=%d total=%d", composite, agree, total)
	}
}

func TestLevelFromCompositeThresholds(t *testing.T) {
	thresholds := ThresholdsConfig{Level1: 1.0, Level2: 2.0, Level3: 3.5}

	cases := []struct {
		composite float64
		wantLevel int
	}{
		{0.5, 0},
		{-0.5, 0},
		{1.0, 1},
		{-1.5, 1},
		{2.0, 2},
		{-3.0, 2},
		{3.5, 3},
		{-10.0, 3},
	}

	for _, tc := range cases {
		if got := levelFromComposite(tc.composite, thresholds); got != tc.wantLevel {
			t.Errorf("levelFromComposite(%.2f) = %d, ожидалось %d", tc.composite, got, tc.wantLevel)
		}
	}
}
