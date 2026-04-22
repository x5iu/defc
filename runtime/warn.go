package defc

import (
	"log"
	"sync"
)

var droppedArgWarnState struct {
	mu      sync.Mutex
	methods map[string]*sync.Once
}

func resetDroppedArgWarnStateForTest() {
	droppedArgWarnState.mu.Lock()
	droppedArgWarnState.methods = nil
	droppedArgWarnState.mu.Unlock()
}

func WarnDroppedArgs(method string, sql string, collected, used int) {
	if collected <= used {
		return
	}
	droppedArgWarnState.mu.Lock()
	if droppedArgWarnState.methods == nil {
		droppedArgWarnState.methods = make(map[string]*sync.Once)
	}
	o, ok := droppedArgWarnState.methods[method]
	if !ok {
		o = new(sync.Once)
		droppedArgWarnState.methods[method] = o
	}
	droppedArgWarnState.mu.Unlock()
	discarded := collected - used
	o.Do(func() {
		preview := sql
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		log.Printf("defc: method %q rendered SQL %q consumed %d argument(s) but %d argument(s) were collected from the method signature; %d argument(s) were discarded. This usually means a method argument is being interpolated into SQL text via {{ .arg }} instead of {{ bind $.arg }} / CONSTBIND ${arg}. See https://github.com/x5iu/defc/issues/17.", method, preview, used, collected, discarded)
	})
}
