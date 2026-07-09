package dao

import (
	"testing"

	"github.com/itering/subscan/model"
	"github.com/itering/subscan/util"
	"github.com/stretchr/testify/assert"
)

func TestMultisigComposerFromEvent(t *testing.T) {
	originalAddressType := util.AddressType
	t.Cleanup(func() { util.AddressType = originalAddressType })
	util.AddressType = "31"
	event := model.ChainEvent{
		Params: model.EventParams{
			{Name: "approving", Value: "0x9c6c86c4936b94ae7772d1045e8f4e36690c84bb6df01c49e167de90902d0817"},
			{Name: "multisig", Value: "0x8898bfb77f32226ec1990ed415ce0f215b05e3d5a872a4e4f4e6a4718be04f85"},
			{Name: "call_hash", Value: "0xf2472a3093ba21e07c3ed2938da1158e3d03d1f9527b2085180288a804317a21"},
		},
	}

	composer, ok := multisigComposerFromEvent(event, "8898bfb77f32226ec1990ed415ce0f215b05e3d5a872a4e4f4e6a4718be04f85")

	assert.True(t, ok)
	assert.Equal(t, "49wYitCNCSrF2swm88DekiF1BQmyi3K5sna3SGjDZ78fsLaL", composer)
}

func TestMultisigComposerFromEventIgnoresOtherAccount(t *testing.T) {
	event := model.ChainEvent{
		Params: model.EventParams{
			{Name: "approving", Value: "0x9c6c86c4936b94ae7772d1045e8f4e36690c84bb6df01c49e167de90902d0817"},
			{Name: "multisig", Value: "0x8898bfb77f32226ec1990ed415ce0f215b05e3d5a872a4e4f4e6a4718be04f85"},
		},
	}

	_, ok := multisigComposerFromEvent(event, "dbc968c19add01bc24568a02b7d02c68e98965fe7df757d8d2733f82bf3aa941")

	assert.False(t, ok)
}
