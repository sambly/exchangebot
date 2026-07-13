// Package toggle - потокобезопасный булев флаг для настроек, которые
// переключаются из телеграм-меню, а читаются из горутин стратегий и шины
// уведомлений. Обработчики telebot выполняются конкурентно, поэтому обычный
// bool в конфиге здесь означает гонку.
package toggle

import "sync"

type Bool struct {
	mu sync.RWMutex
	v  bool
}

func New(v bool) *Bool {
	return &Bool{v: v}
}

func (b *Bool) Get() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.v
}

func (b *Bool) Set(v bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.v = v
}
