# caster

[![GoDoc](https://godoc.org/github.com/guiguan/caster?status.svg)](https://godoc.org/github.com/guiguan/caster)

`caster` is a dead simple and performant message broadcaster for Go with context support. It uses the publisher and subscriber pattern (pubsub) to broadcast messages from a single or multiple source channels to multiple subscriber channels. Subscribers can dynamically join and leave.

## Usage

### Broadcast a Go channel

Suppose the Go channel is:

```go
var srcCh <-chan interface{}
```

We can broadcast the messages coming out of it to multiple subscribers:

```go
c := caster.New(nil)

go func() {
    // subscriber #1
    ch, _ := c.Sub(nil, 1)

    for m := range ch {
        // do anything to the broadcasted message
    }
}()

go func() {
    // subscriber #2
    ch, _ := c.Sub(nil, 1)

    for m := range ch {
        // do anything to the broadcasted message
    }
}()

go func() {
    // publisher
    for m := range srcCh {
        c.Pub(m)
    }

    c.Close()
}()
```

Subscribers can join and leave at any time:

```go
// join
ch1, _ := c.Sub(nil, 1)

// leave
c.Unsub(ch1)

// join with context and automatically leave when the context is canceled
ch2, _ := c.Sub(ctx, 1)

// join with 10 subscriber channel buffer
ch3, _ := c.Sub(ctx, 10)
```

`caster` can associate with a context as well:

```Go
// `c` will be closed when the `ctx` is canceled
c := caster.New(ctx)
```

A boolean value is returned to indicate whether the `caster` is still running or not:

```Go
_, ok := c.Sub(nil, 1)
if !ok {
    // the caster has been closed, do something else
}
```

## License

[MIT](LICENSE)
