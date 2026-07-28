package stat

import "testing"

// Одна и та же сторона несколько раз подряд должна набирать стрик, и
// Confirmed должен срабатывать ровно на пороге, не раньше.
func TestSideConfirmBuildsStreak(t *testing.T) {
	var c SideConfirm

	for i := 0; i < 2; i++ {
		c.Update("BUY")
		if _, ok := c.Confirmed(3); ok {
			t.Fatalf("на сэмпле %d стрик ещё не должен достигать порога 3", i+1)
		}
	}

	c.Update("BUY")
	side, ok := c.Confirmed(3)
	if !ok || side != "BUY" {
		t.Fatalf("после 3 одинаковых сэмплов подряд ожидался Confirmed=BUY, получено side=%q ok=%v", side, ok)
	}
}

// Смена стороны должна сбрасывать стрик полностью, а не продолжать счёт.
func TestSideConfirmResetsOnSideChange(t *testing.T) {
	var c SideConfirm

	c.Update("BUY")
	c.Update("BUY")
	c.Update("BUY")
	if _, ok := c.Confirmed(3); !ok {
		t.Fatal("ожидался Confirmed=true после 3 сэмплов BUY подряд")
	}

	c.Update("SELL")
	if _, ok := c.Confirmed(3); ok {
		t.Fatal("смена стороны должна сбрасывать стрик - Confirmed не должен срабатывать сразу")
	}

	c.Update("SELL")
	c.Update("SELL")
	side, ok := c.Confirmed(3)
	if !ok || side != "SELL" {
		t.Fatalf("после 3 сэмплов SELL подряд (с нуля) ожидался Confirmed=SELL, получено side=%q ok=%v", side, ok)
	}
}

// Пустая сторона ("сигнала сейчас нет") должна сбрасывать стрик так же, как
// и смена на противоположную сторону - а не считаться "нейтральным" сэмплом,
// который можно пропустить без последствий.
func TestSideConfirmResetsOnEmptySide(t *testing.T) {
	var c SideConfirm

	c.Update("BUY")
	c.Update("BUY")
	c.Update("BUY")
	if _, ok := c.Confirmed(3); !ok {
		t.Fatal("ожидался Confirmed=true перед проверкой сброса")
	}

	c.Update("")
	if _, ok := c.Confirmed(3); ok {
		t.Fatal("пустая сторона должна сбрасывать стрик")
	}

	c.Update("BUY")
	if _, ok := c.Confirmed(3); ok {
		t.Fatal("стрик после сброса должен набираться заново, а не продолжаться")
	}
}

// minStreak<=0 не должен требовать вообще ничего - Confirmed=true уже на
// первом сэмпле непустой стороны.
func TestSideConfirmZeroMinStreak(t *testing.T) {
	var c SideConfirm

	c.Update("BUY")
	side, ok := c.Confirmed(0)
	if !ok || side != "BUY" {
		t.Fatalf("minStreak=0: ожидался Confirmed=BUY сразу, получено side=%q ok=%v", side, ok)
	}
}
