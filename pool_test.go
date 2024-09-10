// Pool is no-op under race detector, so all these tests do not work.
//go:build !race

package astjson

import (
	"fmt"
	"sync"
	"testing"
)

func TestParserPool(t *testing.T) {
	var pp ParserPool
	for i := 0; i < 10; i++ {
		p := pp.Get()
		if _, err := p.Parse("null"); err != nil {
			t.Fatalf("cannot parse null: %s", err)
		}
		pp.Put(p)
	}
}

func TestParserPoolMaxSize(t *testing.T) {
	var numNew, numNewLimit int
	ppr := &ParserPool{
		sync.Pool{New: func() interface{} { numNew++; return new(Parser) }},
	}
	pprLimit := &ParserPool{
		sync.Pool{New: func() interface{} { numNewLimit++; return new(Parser) }},
	}

	parse := func(ppr *ParserPool, maxSize int, index int) {
		var json = fmt.Sprintf(`{"%d":"test"}`, index)
		pr := ppr.Get()
		_, _ = pr.Parse(json)
		ppr.PutIfSizeLessThan(pr, maxSize)
	}
	for i := 0; i < 10; i++ {
		parse(ppr, 0, i)
		parse(pprLimit, 1, i)
	}

	if numNew != 1 {
		t.Fatalf("Expected exactly 1 calls to Pool New with no Max Size (not %d)", numNew)
	}

	if numNewLimit != 10 {
		t.Fatalf("Expected exactly 10 calls to Pool with a Max Size (not %d)", numNewLimit)
	}
}
