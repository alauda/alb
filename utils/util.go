// any function defined here MUST be pure function.
package utils

import "sync"

func CleanSyncMap(m sync.Map) {
	m.Range(func(key interface{}, value interface{}) bool {
		m.Delete(key)
		return true
	})
}
