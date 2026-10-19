// This nested module keeps frontend dependencies out of the root Go module's
// `./...` package scan. The web directory is owned by npm tooling, not Go.
module github.com/NSObjects/echo-admin/web

go 1.26
