package entrysetup

import (
	"time"

	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/stat"
)

const (
	// wallsConfirmSampleInterval - не чаще раза в сколько сэмплировать
	// устойчивость стороны стен на пару. Тот же порядок, что и у
	// imbalanceSampleInterval в depth, и совпадает с интервалом опроса
	// вкладки "Потенциал" на фронте - конкретное значение здесь не
	// принципиально, важно, чтобы сэмплы не набивались быстрее, чем стакан
	// успевает реально измениться.
	wallsConfirmSampleInterval = 10 * time.Second

	// wallsConfirmMinStreak - см. imbalanceConfirmMinStreak в depth: тот же
	// компромисс между "не спуфинг/шум" и "успевает подтвердиться, пока
	// реально держится".
	wallsConfirmMinStreak = 3
)

// sampleWallsConfirm обновляет счётчик устойчивости стороны стен для pair, не
// чаще раза в wallsConfirmSampleInterval (повторные вызовы внутри этого окна -
// no-op). Вызывается из GetAllStrengthComponents на уже посчитанных walls, без
// повторного похода в стакан.
func (s *AssetsSetup) sampleWallsConfirm(pair string, walls Walls) {
	now := time.Now()

	s.wallsConfirmMu.Lock()
	defer s.wallsConfirmMu.Unlock()

	if s.wallsConfirm == nil {
		s.wallsConfirm = make(map[string]*stat.SideConfirm)
		s.wallsConfirmNextSampleAt = make(map[string]time.Time)
	}

	if next, ok := s.wallsConfirmNextSampleAt[pair]; ok && now.Before(next) {
		return
	}
	s.wallsConfirmNextSampleAt[pair] = now.Add(wallsConfirmSampleInterval)

	confirm, ok := s.wallsConfirm[pair]
	if !ok {
		confirm = &stat.SideConfirm{}
		s.wallsConfirm[pair] = confirm
	}

	side, _ := wallsSide(walls) // ok=false -> side="" - ровно то, что нужно Update
	confirm.Update(string(side))
}

// GetWallsConfirmedSide - сторона ближней стены (см. wallsSide), если она
// держится минимум wallsConfirmMinStreak сэмплов подряд (см.
// sampleWallsConfirm). confirmed=false - либо сторона ещё не набрала стрик,
// либо по паре ещё не было ни одного сэмпла (GetAllStrengthComponents ни
// разу не вызывался).
func (s *AssetsSetup) GetWallsConfirmedSide(pair string) (side order.SideType, confirmed bool) {
	s.wallsConfirmMu.Lock()
	defer s.wallsConfirmMu.Unlock()

	confirm, ok := s.wallsConfirm[pair]
	if !ok {
		return "", false
	}

	str, ok := confirm.Confirmed(wallsConfirmMinStreak)
	if !ok {
		return "", false
	}
	return order.SideType(str), true
}
