package entrysetup

import (
	"testing"
	"time"

	"github.com/sambly/exchangebot/internal/order"
)

func TestWallsSidePicksCloserWall(t *testing.T) {
	// support ближе -> BUY
	walls := Walls{
		HasSupport:    true,
		Support:       Wall{DistancePercent: 0.5},
		HasResistance: true,
		Resistance:    Wall{DistancePercent: 2.0},
	}
	side, ok := wallsSide(walls)
	if !ok || side != order.SideTypeBuy {
		t.Fatalf("ожидался BUY (support ближе), получено side=%v ok=%v", side, ok)
	}

	// resistance ближе -> SELL
	walls = Walls{
		HasSupport:    true,
		Support:       Wall{DistancePercent: 3.0},
		HasResistance: true,
		Resistance:    Wall{DistancePercent: 1.0},
	}
	side, ok = wallsSide(walls)
	if !ok || side != order.SideTypeSell {
		t.Fatalf("ожидался SELL (resistance ближе), получено side=%v ok=%v", side, ok)
	}
}

func TestWallsSideRequiresBothWalls(t *testing.T) {
	if _, ok := wallsSide(Walls{HasSupport: true, Support: Wall{DistancePercent: 1}}); ok {
		t.Fatal("без Resistance wallsSide должен вернуть ok=false")
	}
}

// Одна выборка не должна подтверждать сторону - нужно, чтобы она держалась
// wallsConfirmMinStreak сэмплов подряд (та же защита от спуфинга/шума, что и
// у имбаланса в depth).
func TestSampleWallsConfirmRequiresStreak(t *testing.T) {
	bids := levels(100.0, -0.1, 20, 1.0, 1, 50.0) // support ближе (BUY)
	asks := levels(100.1, 0.1, 20, 1.0, 10, 50.0) // resistance дальше

	s := setupWithBook(t, "BTCUSDT", bids, asks)
	walls, ok := s.GetWalls("BTCUSDT")
	if !ok {
		t.Fatal("GetWalls должен вернуть ok=true")
	}

	sampleWallsConfirmNow(s, "BTCUSDT", walls)
	if _, confirmed := s.GetWallsConfirmedSide("BTCUSDT"); confirmed {
		t.Fatal("после одного сэмпла сторона не должна считаться подтверждённой")
	}

	sampleWallsConfirmNow(s, "BTCUSDT", walls)
	sampleWallsConfirmNow(s, "BTCUSDT", walls)

	side, confirmed := s.GetWallsConfirmedSide("BTCUSDT")
	if !confirmed || side != order.SideTypeBuy {
		t.Fatalf("после %d сэмплов подряд ожидался Confirmed=BUY, получено side=%v confirmed=%v",
			wallsConfirmMinStreak, side, confirmed)
	}
}

// Разворот стороны должен сбрасывать стрик, а не наследовать его от прежней,
// уже неактуальной стороны.
func TestSampleWallsConfirmResetsOnFlip(t *testing.T) {
	bidsBuy := levels(100.0, -0.1, 20, 1.0, 1, 50.0)
	asksBuy := levels(100.1, 0.1, 20, 1.0, 10, 50.0)

	s := setupWithBook(t, "BTCUSDT", bidsBuy, asksBuy)
	wallsBuy, ok := s.GetWalls("BTCUSDT")
	if !ok {
		t.Fatal("GetWalls должен вернуть ok=true")
	}

	for i := 0; i < wallsConfirmMinStreak; i++ {
		sampleWallsConfirmNow(s, "BTCUSDT", wallsBuy)
	}
	if _, confirmed := s.GetWallsConfirmedSide("BTCUSDT"); !confirmed {
		t.Fatal("ожидался Confirmed=BUY перед проверкой сброса")
	}

	// Зеркальная конфигурация - resistance теперь ближе (SELL).
	wallsSell := Walls{
		HasSupport:    true,
		Support:       Wall{DistancePercent: 3.0},
		HasResistance: true,
		Resistance:    Wall{DistancePercent: 1.0},
	}
	sampleWallsConfirmNow(s, "BTCUSDT", wallsSell)

	if _, confirmed := s.GetWallsConfirmedSide("BTCUSDT"); confirmed {
		t.Fatal("после разворота стороны подтверждение должно сброситься")
	}
}

// sampleWallsConfirmNow обходит троттлинг wallsConfirmSampleInterval, чтобы в
// тесте не ждать реального времени между сэмплами.
func sampleWallsConfirmNow(s *AssetsSetup, pair string, walls Walls) {
	s.wallsConfirmMu.Lock()
	if s.wallsConfirmNextSampleAt != nil {
		s.wallsConfirmNextSampleAt[pair] = time.Now().Add(-time.Second)
	}
	s.wallsConfirmMu.Unlock()

	s.sampleWallsConfirm(pair, walls)
}
