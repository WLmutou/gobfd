package gobfd

import (
	"go.uber.org/zap"
)

var (
	logger  *zap.Logger
	slogger *zap.SugaredLogger
)

func init() {
	logger = zap.L()
	slogger = zap.S()

}


