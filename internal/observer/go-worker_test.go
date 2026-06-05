package observer

import (
	"testing"

	"github.com/itering/subscan/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginExtrinsicArgsReadsExtrinsicIndex(t *testing.T) {
	var args pluginExtrinsicArgs

	err := util.UnmarshalAny(&args, map[string]interface{}{
		"extrinsic_index": "9716376-2",
		"plugin_name":     "balance",
	})

	require.NoError(t, err)
	assert.Equal(t, "9716376-2", args.ExtrinsicIndex)
	assert.Equal(t, "balance", args.PluginName)
}

func TestPluginExtrinsicArgsDoesNotReadEventIndex(t *testing.T) {
	var args pluginExtrinsicArgs

	err := util.UnmarshalAny(&args, map[string]interface{}{
		"event_index": "9716376-3",
		"plugin_name": "balance",
	})

	require.NoError(t, err)
	assert.Empty(t, args.ExtrinsicIndex)
	assert.Equal(t, "balance", args.PluginName)
}
