package defc

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"strings"
	"sync"
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
	type stKind int
	const (
		stNormal stKind = iota
		stSQuote
		stDQuote
		stLineCom
		stBlockCom
	)
	var b strings.Builder
	st := stNormal
	dquoteStart := -1
	pendingSpace := false
	emitSpace := func() {
		if b.Len() > 0 {
			pendingSpace = true
		}
	}
	flushSpace := func() {
		if pendingSpace && b.Len() > 0 {
			b.WriteByte(' ')
		}
		pendingSpace = false
	}
	writeByte := func(c byte) {
		flushSpace()
		b.WriteByte(c)
	}
	writeStr := func(x string) {
		flushSpace()
		b.WriteString(x)
	}
	sqlIdentByte := func(c byte) bool {
		return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
	}
	prevNotIdent := func(i int) bool {
		if i == 0 {
			return true
		}
		return !sqlIdentByte(sql[i-1])
	}
	nextNotIdent := func(i int) bool {
		if i >= len(sql) {
			return true
		}
		return !sqlIdentByte(sql[i])
	}
	isNumStart := func(i int) bool {
		c := sql[i]
		if c >= '0' && c <= '9' {
			return true
		}
		return c == '.' && i+1 < len(sql) && sql[i+1] >= '0' && sql[i+1] <= '9'
	}
	consumeNumber := func(i int) int {
		i++
		for i < len(sql) {
			c := sql[i]
			if (c >= '0' && c <= '9') || c == '.' {
				i++
				continue
			}
			if c == 'e' || c == 'E' {
				i++
				if i < len(sql) && (sql[i] == '+' || sql[i] == '-') {
					i++
				}
				continue
			}
			break
		}
		return i
	}
	dquoteCloses := func(i int) bool {
		bs := 0
		for j := i - 1; j >= 0 && sql[j] == '\\'; j-- {
			bs++
		}
		return bs%2 == 0
	}
	for i := 0; i < len(sql); {
		c := sql[i]
		switch st {
		case stLineCom:
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			if i < len(sql) {
				i++
			}
			st = stNormal
			emitSpace()
			continue
		case stBlockCom:
			for i < len(sql) {
				if sql[i] == '*' && i+1 < len(sql) && sql[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			st = stNormal
			emitSpace()
			continue
		case stSQuote:
			if c == '\'' {
				if i+1 < len(sql) && sql[i+1] == '\'' {
					i += 2
					continue
				}
				writeStr("?")
				i++
				st = stNormal
				continue
			}
			if c == '\\' && i+1 < len(sql) {
				i += 2
				continue
			}
			i++
			continue
		case stDQuote:
			if c == '\\' && i+1 < len(sql) {
				i += 2
				continue
			}
			if c == '"' {
				if !dquoteCloses(i) {
					if i+1 < len(sql) {
						i += 2
					} else {
						i++
					}
					continue
				}
				if i+1 < len(sql) && sql[i+1] == '"' {
					i += 2
					continue
				}
				writeStr(sql[dquoteStart : i+1])
				i++
				st = stNormal
				dquoteStart = -1
				continue
			}
			i++
			continue
		case stNormal:
			if c == '-' && i+1 < len(sql) && sql[i+1] == '-' {
				i += 2
				st = stLineCom
				continue
			}
			if c == '/' && i+1 < len(sql) && sql[i+1] == '*' {
				i += 2
				st = stBlockCom
				continue
			}
			if c == '\'' {
				flushSpace()
				i++
				st = stSQuote
				continue
			}
			if c == '"' {
				flushSpace()
				dquoteStart = i
				i++
				st = stDQuote
				continue
			}
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				i++
				for i < len(sql) {
					cc := sql[i]
					if cc != ' ' && cc != '\t' && cc != '\n' && cc != '\r' {
						break
					}
					i++
				}
				emitSpace()
				continue
			}
			if isNumStart(i) && prevNotIdent(i) {
				j := consumeNumber(i)
				if nextNotIdent(j) {
					i = j
					writeStr("?")
					continue
				}
			}
			if c >= 'A' && c <= 'Z' {
				writeByte(c + 32)
			} else {
				writeByte(c)
			}
			i++
		}
	}
	if st == stSQuote {
		flushSpace()
		b.WriteByte('?')
	} else if st == stDQuote {
		flushSpace()
		if dquoteStart >= 0 {
			b.WriteString(sql[dquoteStart:])
		}
	}
	return strings.TrimSpace(b.String())
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
