package state

import (
	"sync"

	. "alauda.io/alb2/controller/modules"
)

// 维护一些全局的alb的状态 目前只有优雅退出的特性用到了

type State struct {
	phase AlbPhase
	lock  sync.Mutex
}

var instance *State
var once sync.Once

func GetState() *State {
	once.Do(func() {
		instance = &State{
			phase: PhaseStarting,
			lock:  sync.Mutex{},
		}
	})
	return instance
}

func (s *State) GetPhase() AlbPhase {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.phase
}

func (s *State) SetPhase(p AlbPhase) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.phase = p
}
