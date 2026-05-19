package contractschema

import (
	"sync"
	"testing"

	"github.com/itering/subscan/plugins/evm/dao"
	"gorm.io/gorm/schema"
)

func TestContractTableNameForValueAndPointerModels(t *testing.T) {
	for name, model := range map[string]interface{}{
		"value":   dao.Contract{},
		"pointer": &dao.Contract{},
	} {
		t.Run(name, func(t *testing.T) {
			parsed, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Table != "evm_contracts" {
				t.Fatalf("Contract table = %q, want evm_contracts", parsed.Table)
			}
		})
	}
}
