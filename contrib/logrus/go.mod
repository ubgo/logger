module github.com/ubgo/logger/contrib/logrus

go 1.24

replace github.com/ubgo/logger => ../..

require (
	github.com/sirupsen/logrus v1.9.4
	github.com/ubgo/logger v0.0.0-00010101000000-000000000000
)

require golang.org/x/sys v0.13.0 // indirect
