package balance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandsIncludesBridgeOnlyBackfill(t *testing.T) {
	var commandNames []string
	for _, command := range New().Commands() {
		commandNames = append(commandNames, command.Name)
	}

	assert.Contains(t, commandNames, "InitTransfer")
	assert.Contains(t, commandNames, "BackfillOmniBridgeTransfers")
}
