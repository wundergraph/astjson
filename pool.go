package astjson

import (
	"sync"
)

// ParserPool may be used for pooling Parsers for similarly typed JSONs.
type ParserPool struct {
	pool sync.Pool
}

// Get returns a Parser from pp.
//
// The Parser must be Put to pp after use.
func (pp *ParserPool) Get() *Parser {
	v := pp.pool.Get()
	if v == nil {
		return &Parser{}
	}
	return v.(*Parser)
}

// Put returns p to pp.
//
// p and objects recursively returned from p cannot be used after p
// is put into pp.
func (pp *ParserPool) Put(p *Parser) {
	pp.pool.Put(p)
}

// PutIfSizeLessThan PutIfLessThan Put returns p to pp only if the number of values in the cache is less than maxSize.
// If set to <= 0, no size limit is applied.
//
// p and objects recursively returned from p cannot be used after p is put into pp or released
func (pp *ParserPool) PutIfSizeLessThan(p *Parser, maxSize int) {
	// Release the parser if the cache is too big
	if maxSize > 0 && cap(p.c.vs) > maxSize {
		return
	}

	pp.pool.Put(p)
}

// ArenaPool may be used for pooling Arenas for similarly typed JSONs.
type ArenaPool struct {
	pool sync.Pool
}

// Get returns an Arena from ap.
//
// The Arena must be Put to ap after use.
func (ap *ArenaPool) Get() *Arena {
	v := ap.pool.Get()
	if v == nil {
		return &Arena{}
	}
	return v.(*Arena)
}

// Put returns a to ap.
//
// a and objects created by a cannot be used after a is put into ap.
func (ap *ArenaPool) Put(a *Arena) {
	ap.pool.Put(a)
}
