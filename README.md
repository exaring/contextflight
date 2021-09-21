![CI status](https://github.com/exaring/contextflight/actions/workflows/go.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/exaring/contextflight)](https://goreportcard.com/report/github.com/exaring/contextflight)

# contextflight

`contextflight` is a thin wrapper around [singleflight](https://pkg.go.dev/golang.org/x/sync/singleflight) that adds
context handling.

It works like `singleflight`, with the addition that the provided function receives a `context.Context`, which will
be cancelled when _all_ waiting callers' contexts are cancelled.

This allows for correctly cancelling an expensive operation inside of singleflight only when _all_ requesters have
canceled their requests.
