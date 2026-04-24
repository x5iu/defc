package defc

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"sync"

	tok "github.com/x5iu/defc/runtime/token"
)

var droppedArgWarnState struct {
	mu   sync.Mutex
	keys map[string]*sync.Once
}

func resetDroppedArgWarnStateForTest() {
	droppedArgWarnState.mu.Lock()
	droppedArgWarnState.keys = nil
	droppedArgWarnState.mu.Unlock()
}

func shapeDigest(sql string) string {
	sum := sha256.Sum256([]byte(normalizeSQLShape(sql)))
	return hex.EncodeToString(sum[:])[:16]
}

func normalizeSQLShape(sql string) string {
	return tok.NormalizeShape(sql)
}

func WarnDroppedArgs(method string, sql string, collected, used int) {
	if collected <= used {
		return
	}
	key := method + "\x00" + shapeDigest(sql)
	droppedArgWarnState.mu.Lock()
	if droppedArgWarnState.keys == nil {
		droppedArgWarnState.keys = make(map[string]*sync.Once)
	}
	o, ok := droppedArgWarnState.keys[key]
	if !ok {
		o = new(sync.Once)
		droppedArgWarnState.keys[key] = o
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
