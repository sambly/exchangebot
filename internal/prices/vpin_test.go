package prices

import (
	"testing"
	"time"
)

// Без бакетов и без набранной вручную истории GetVPINZScore должен честно
// отдавать ready=false, а не 0 как будто посчитано.
func TestGetVPINZScoreNotReadyWithoutBuckets(t *testing.T) {
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute}, &stubRepo{})

	if _, _, ready := ap.GetVPINZScore("BTCUSDT"); ready {
		t.Fatal("без единого закоммиченного бакета GetVPINZScore должен вернуть ready=false")
	}
}

// Бакет не должен коммититься, пока не набран целевой объём (vpinBucketMultiplier
// "типичных" объёмов свечи) - при постоянном объёме 100 за свечу это ровно
// vpinBucketMultiplier свечей. История сидируется заранее на vpinMinSamples-1
// нейтральных значений (см. TestVPINConfirmedSideRequiresStreak) - иначе
// коммит был бы vpinMinSamples-м по счёту сэмплом истории и GetVPINZScore
// честно отдал бы ready=false (см. Go-комментарий у GetVPINZScore).
func TestRecordVPINAccumulatesThenCommits(t *testing.T) {
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute}, &stubRepo{})
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	state := ap.vpin["BTCUSDT"]
	for i := 0; i < vpinMinSamples-1; i++ {
		state.history.Add(0)
	}

	target := int(vpinBucketMultiplier)
	for i := 0; i < target-1; i++ {
		ap.recordVPIN("BTCUSDT", base.Add(time.Duration(i)*time.Minute), 100, 100)
	}
	if _, _, ready := ap.GetVPINZScore("BTCUSDT"); ready {
		t.Fatal("бакет не должен закоммититься раньше набора целевого объёма")
	}

	ap.recordVPIN("BTCUSDT", base.Add(time.Duration(target-1)*time.Minute), 100, 100)

	value, _, ready := ap.GetVPINZScore("BTCUSDT")
	if !ready {
		t.Fatalf("после %d полностью покупных свечей бакет должен закоммититься", target)
	}
	if value < 0.999 || value > 1.001 {
		t.Fatalf("value = %.4f, ожидалось ~1.0 (100%% объёма бакета - покупки)", value)
	}
}

// Ровно половина объёма - покупки, половина - продажи: value должен быть ~0,
// а не смещён в какую-то сторону.
func TestRecordVPINNeutralSplitGivesZero(t *testing.T) {
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute}, &stubRepo{})
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	state := ap.vpin["BTCUSDT"]
	for i := 0; i < vpinMinSamples-1; i++ {
		state.history.Add(0)
	}

	target := int(vpinBucketMultiplier)
	for i := 0; i < target; i++ {
		ap.recordVPIN("BTCUSDT", base.Add(time.Duration(i)*time.Minute), 50, 100)
	}

	value, _, ready := ap.GetVPINZScore("BTCUSDT")
	if !ready {
		t.Fatal("бакет должен был закоммититься")
	}
	if value < -0.001 || value > 0.001 {
		t.Fatalf("value = %.4f, ожидалось ~0 (нейтральный сплит)", value)
	}
}

// GetVPINZScore должен отдавать ready=false, пока история не набрала
// vpinMinSamples закоммиченных бакетов - даже если бакет уже коммитится
// (committed=true). Это тот же класс бага, что был найден и исправлен для
// RelativeStrength: без этой проверки первые vpinMinSamples-1 коммитов
// отдавали бы ready=true с бессмысленным z=0.
func TestGetVPINZScoreNotReadyBeforeMinSamples(t *testing.T) {
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute}, &stubRepo{})
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	fillBullishBucket(ap, "BTCUSDT", base, 0)

	if _, _, ready := ap.GetVPINZScore("BTCUSDT"); ready {
		t.Fatal("после единственного закоммиченного бакета (меньше vpinMinSamples) GetVPINZScore должен отдавать ready=false")
	}
}

// Повторная свеча с тем же временем (та же дедупликация, что и у
// window.pushNewest) не должна учитываться дважды.
func TestRecordVPINDeduplicatesSameCandleTime(t *testing.T) {
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute}, &stubRepo{})
	at := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	ap.recordVPIN("BTCUSDT", at, 100, 100)
	ap.recordVPIN("BTCUSDT", at, 100, 100) // повтор той же свечи

	state := ap.vpin["BTCUSDT"]
	if state.bucketTotalVolume != 100 {
		t.Fatalf("повтор свечи с тем же временем не должен добавляться повторно, bucketTotalVolume = %.2f, ожидалось 100", state.bucketTotalVolume)
	}
}

// Неизвестная пара и нулевой объём не должны паниковать (в т.ч. деление на 0).
func TestRecordVPINIgnoresUnknownPairAndZeroVolume(t *testing.T) {
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute}, &stubRepo{})
	now := time.Now()

	ap.recordVPIN("ETHUSDT", now, 10, 10)
	ap.recordVPIN("BTCUSDT", now, 10, 0)

	if _, _, ready := ap.GetVPINZScore("BTCUSDT"); ready {
		t.Fatal("нулевой объём не должен был запустить коммит бакета")
	}
}

// fillBullishBucket закрывает ОДИН бакет (vpinBucketMultiplier свечей по 100
// объёма, полностью покупки) начиная с минуты bucketIndex*vpinBucketMultiplier
// от base - соседние бакеты не пересекаются по времени свечей.
func fillBullishBucket(ap *AssetsPrices, pair string, base time.Time, bucketIndex int) {
	perBucket := int(vpinBucketMultiplier)
	for i := 0; i < perBucket; i++ {
		minute := bucketIndex*perBucket + i
		ap.recordVPIN(pair, base.Add(time.Duration(minute)*time.Minute), 100, 100)
	}
}

// fillBearishBucket - зеркально fillBullishBucket, но полностью продажи.
func fillBearishBucket(ap *AssetsPrices, pair string, base time.Time, bucketIndex int) {
	perBucket := int(vpinBucketMultiplier)
	for i := 0; i < perBucket; i++ {
		minute := bucketIndex*perBucket + i
		ap.recordVPIN(pair, base.Add(time.Duration(minute)*time.Minute), 0, 100)
	}
}

// Один резко бычий бакет не должен считаться "подтверждённым" сразу - нужно,
// чтобы он держался vpinConfirmMinStreak закоммиченных бакетов подряд. История
// сидируется нейтральными нулями заранее (см. depth_test.go - тот же приём):
// без vpinMinSamples накопленных сэмплов z-score всегда 0 независимо от value.
func TestVPINConfirmedSideRequiresStreak(t *testing.T) {
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute}, &stubRepo{})
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	state := ap.vpin["BTCUSDT"]
	for i := 0; i < vpinMinSamples; i++ {
		state.history.Add(0)
	}

	fillBullishBucket(ap, "BTCUSDT", base, 0)
	if _, confirmed := ap.GetVPINConfirmedSide("BTCUSDT"); confirmed {
		t.Fatal("после одного закоммиченного бакета сторона не должна считаться подтверждённой")
	}

	fillBullishBucket(ap, "BTCUSDT", base, 1)
	fillBullishBucket(ap, "BTCUSDT", base, 2)

	side, confirmed := ap.GetVPINConfirmedSide("BTCUSDT")
	if !confirmed || side != "BUY" {
		t.Fatalf("после %d бычьих бакетов подряд ожидался Confirmed=BUY, получено side=%q confirmed=%v",
			vpinConfirmMinStreak, side, confirmed)
	}
}

// Резкий разворот потока должен сбрасывать стрик - подтверждение не должно
// "унаследоваться" от предыдущей, уже неактуальной стороны.
func TestVPINConfirmedSideResetsOnFlip(t *testing.T) {
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute}, &stubRepo{})
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	state := ap.vpin["BTCUSDT"]
	for i := 0; i < vpinMinSamples; i++ {
		state.history.Add(0)
	}

	fillBullishBucket(ap, "BTCUSDT", base, 0)
	fillBullishBucket(ap, "BTCUSDT", base, 1)
	fillBullishBucket(ap, "BTCUSDT", base, 2)
	if _, confirmed := ap.GetVPINConfirmedSide("BTCUSDT"); !confirmed {
		t.Fatal("ожидался Confirmed=BUY перед проверкой сброса")
	}

	fillBearishBucket(ap, "BTCUSDT", base, 3)

	if _, confirmed := ap.GetVPINConfirmedSide("BTCUSDT"); confirmed {
		t.Fatal("после медвежьего бакета подтверждение BUY должно сброситься")
	}
}
