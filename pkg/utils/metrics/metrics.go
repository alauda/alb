package metrics

import "sync"

var (
	_globalMu sync.RWMutex
	_globalM  *metrics = nil
)

type metrics struct {
	data map[string]float64
}

func init() {
	_globalMu.Lock()
	defer _globalMu.Unlock()
	if _globalM != nil {
		return
	}

	_globalM = &metrics{data: map[string]float64{}}
}

// TODO 用 otel span..

func Write(key string, value float64) {
	_globalMu.Lock()
	defer _globalMu.Unlock()
	_globalM.data[key] = value
}

func Read() map[string]float64 {
	_globalMu.RLock()
	defer _globalMu.RUnlock()
	return _globalM.data
}
