package commands

import "github.com/spezifisch/stmps/logger"

type CommandContext struct {
	Logger      *logger.Logger
	CurrentPage string
	// Other UI or state fields
}
