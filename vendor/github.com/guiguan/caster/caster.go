/*
 * @Author: guiguan
 * @Date:   2019-09-19T00:53:54+10:00
 * @Last modified by:   guiguan
 * @Last modified time: 2019-11-04T16:17:09+11:00
 */

// Package caster implements a dead simple and performant message broadcaster (pubsub) library
package caster

import (
	"context"
)

type operator int

const (
	opPub operator = iota
	opTryPub
	opSub
	opUnsub
	opClose
)

type operation struct {
	operator operator
	operand  interface{}
}

type subInfo struct {
	ch  chan interface{}
	ctx context.Context
}

// Caster represents a message broadcaster
type Caster struct {
	done chan struct{}
	op   chan operation
}

// New creates a new caster
func New(ctx context.Context) *Caster {
	if ctx == nil {
		ctx = context.Background()
	}

	c := &Caster{
		done: make(chan struct{}),
		op:   make(chan operation),
	}

	go func() {
		subs := map[chan interface{}]context.Context{}

		checkCtx := func(sCh chan interface{}, sCtx context.Context) bool {
			select {
			case <-sCtx.Done():
				delete(subs, sCh)
				close(sCh)
				return false
			default:
				return true
			}
		}

	topLoop:
		for {
			select {
			case <-ctx.Done():
				break topLoop
			case o := <-c.op:
				switch o.operator {
				case opPub:
					for sCh, sCtx := range subs {
						if !checkCtx(sCh, sCtx) {
							continue
						}

						select {
						case sCh <- o.operand:
						}
					}
				case opTryPub:
					for sCh, sCtx := range subs {
						if !checkCtx(sCh, sCtx) {
							continue
						}

						select {
						case sCh <- o.operand:
						default:
						}
					}
				case opSub:
					sIn := o.operand.(subInfo)
					subs[sIn.ch] = sIn.ctx
				case opUnsub:
					sCh := o.operand.(chan interface{})
					delete(subs, sCh)
					close(sCh)
				case opClose:
					break topLoop
				}
			}
		}

		for sCh := range subs {
			close(sCh)
		}

		close(c.done)
	}()

	return c
}

// Done returns a done channel that is closed when current caster is closed
func (c *Caster) Done() <-chan struct{} {
	return c.done
}

// Close closes current caster and all subscriber channels. Ok value indicates whether the operation
// is performed or not. When current caster is closed, it stops receiving further operations and the
// operation won't be performed.
func (c *Caster) Close() (ok bool) {
	select {
	case <-c.done:
		return false
	case c.op <- operation{
		operator: opClose,
	}:
		return true
	}
}

// Sub subscribes to current caster and returns a new channel with the given buffer for the
// subscriber to receive the broadcasting message. When the given ctx is canceled, current caster
// will unsubscribe the subscriber channel and close it. Ok value indicates whether the operation is
// performed or not. When current caster is closed, it stops receiving further operations and the
// operation won't be performed. A closed receiver channel will be returned if ok is false.
func (c *Caster) Sub(ctx context.Context, capacity uint) (sCh chan interface{}, ok bool) {
	if ctx == nil {
		ctx = context.Background()
	}

	sCh = make(chan interface{}, capacity)

	select {
	case <-ctx.Done():
		close(sCh)
	case <-c.done:
		close(sCh)
	case c.op <- operation{
		operator: opSub,
		operand: subInfo{
			ch:  sCh,
			ctx: ctx,
		},
	}:
	}

	ok = true
	return
}

// Unsub unsubscribes the given subscriber channel from current caster and closes it. Ok value
// indicates whether the operation is performed or not. When current caster is closed, it stops
// receiving further operations and the operation won't be performed.
func (c *Caster) Unsub(subCh chan interface{}) (ok bool) {
	select {
	case <-c.done:
		return false
	case c.op <- operation{
		operator: opUnsub,
		operand:  subCh,
	}:
		return true
	}
}

// Pub publishes the given message to current caster, so the caster in turn broadcasts the message
// to all subscriber channels. Ok value indicates whether the operation is performed or not. When
// current caster is closed, it stops receiving further operations and the operation won't be
// performed.
func (c *Caster) Pub(msg interface{}) (ok bool) {
	select {
	case <-c.done:
		return false
	case c.op <- operation{
		operator: opPub,
		operand:  msg,
	}:
		return true
	}
}

// TryPub publishes the given message to current caster, so the caster in turn broadcasts the
// message to all subscriber channels without blocking on waiting for channels to be ready for
// receiving. Ok value indicates whether the operation is performed or not. When current caster is
// closed, it stops receiving further operations and the operation won't be performed.
func (c *Caster) TryPub(msg interface{}) (ok bool) {
	select {
	case <-c.done:
		return false
	case c.op <- operation{
		operator: opTryPub,
		operand:  msg,
	}:
		return true
	}
}
