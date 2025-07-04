package utils

import (
	"errors"
	"strings"
	"sync"

	tele "gopkg.in/telebot.v3"
)

// CallbackRegistry хранит обработчики для кнопок inline (динамические, можно создать и удалить)
// как из menuManager так и глобально со структуры Telegram
type CallbackRegistry struct {
	mu       sync.RWMutex
	handlers map[string]tele.HandlerFunc
}

func NewCallbackRegistry() *CallbackRegistry {
	return &CallbackRegistry{
		handlers: make(map[string]tele.HandlerFunc),
	}
}

func (r *CallbackRegistry) Register(unique string, handler tele.HandlerFunc) error {
	if unique == "" {
		return errors.New("callback button must have non-empty Unique field")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[unique] = handler
	return nil
}

func (r *CallbackRegistry) Unregister(unique string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, unique)
}

func (r *CallbackRegistry) GetHandler(unique string) (tele.HandlerFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[unique]

	return h, ok
}

// prefix: buy_, account_, strategy_
func (r *CallbackRegistry) ClearPrefix(prefix string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k := range r.handlers {
		if strings.HasPrefix(k, prefix) {
			delete(r.handlers, k)
		}
	}
}

func (r *CallbackRegistry) ClearAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers = make(map[string]tele.HandlerFunc)
}
