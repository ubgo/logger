module github.com/ubgo/logger/contrib/zerolog

go 1.24

replace github.com/ubgo/logger => ../..

require (
	github.com/rs/zerolog v1.35.1
	github.com/ubgo/logger v0.0.0-00010101000000-000000000000
)

require (
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.29.0 // indirect
)
