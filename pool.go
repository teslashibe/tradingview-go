package tradingview

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
)

// connPool is a bounded pool of authenticated WebSocket conns. A fetch
// acquires exclusive use of a conn, runs to completion, and releases.
// Idle conns are reaped after Config.IdleConnTimeout.
//
// Sizing is enforced by a semaphore: at most Config.PoolSize fetches
// run concurrently. The pool itself stores conns that are free for
// reuse; if no free conn is available when a fetch acquires its slot,
// a fresh one is dialed.
type connPool struct {
	cfg Config
	log Logger
	sem *semaphore.Weighted

	mu     sync.Mutex
	free   []*pooledConn
	closed bool

	stop chan struct{}
}

type pooledConn struct {
	c         *conn
	idleSince time.Time
}

func newConnPool(cfg Config, log Logger) *connPool {
	p := &connPool{
		cfg:  cfg,
		log:  log,
		sem:  semaphore.NewWeighted(int64(cfg.PoolSize)),
		stop: make(chan struct{}),
	}
	go p.reapLoop()
	return p
}

// acquire reserves a slot and returns a conn (reusing a free one if
// available, otherwise dialing). The slot is released by release().
func (p *connPool) acquire(ctx context.Context) (*conn, error) {
	if err := p.sem.Acquire(ctx, 1); err != nil {
		return nil, newErr(CodeUpstreamTimeout, "pool acquire canceled", err)
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		p.sem.Release(1)
		return nil, newErr(CodeClosed, "pool closed", nil)
	}
	if n := len(p.free); n > 0 {
		pc := p.free[n-1]
		p.free = p.free[:n-1]
		p.mu.Unlock()
		return pc.c, nil
	}
	p.mu.Unlock()

	c, err := dial(ctx, p.cfg, p.log)
	if err != nil {
		p.sem.Release(1)
		return nil, err
	}
	return c, nil
}

// release returns a conn to the pool. If healthy is false the conn is
// closed; either way the slot is freed.
func (p *connPool) release(c *conn, healthy bool) {
	if c == nil {
		p.sem.Release(1)
		return
	}
	p.mu.Lock()
	closing := p.closed || !healthy
	if !closing {
		p.free = append(p.free, &pooledConn{c: c, idleSince: time.Now()})
	}
	p.mu.Unlock()
	if closing {
		_ = c.close()
	}
	p.sem.Release(1)
}

func (p *connPool) reapLoop() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-t.C:
			p.reapIdle()
		}
	}
}

func (p *connPool) reapIdle() {
	cutoff := time.Now().Add(-p.cfg.IdleConnTimeout)
	p.mu.Lock()
	keep := p.free[:0]
	var drop []*pooledConn
	for _, pc := range p.free {
		if pc.idleSince.Before(cutoff) {
			drop = append(drop, pc)
		} else {
			keep = append(keep, pc)
		}
	}
	p.free = keep
	p.mu.Unlock()
	for _, pc := range drop {
		_ = pc.c.close()
	}
}

func (p *connPool) close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	free := p.free
	p.free = nil
	p.mu.Unlock()
	close(p.stop)
	for _, pc := range free {
		_ = pc.c.close()
	}
	return nil
}
