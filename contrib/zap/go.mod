module github.com/ubgo/logger/contrib/zap

go 1.24

replace github.com/ubgo/logger => ../..

require (
	github.com/ubgo/logger v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.28.0
)

require go.uber.org/multierr v1.10.0 // indirect
