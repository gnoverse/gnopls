// This file handles `gno` interrealm specificity by directly dealing with AST.
// Although it is somewhat hacky, it functions well for the current requirements.

package cache

import (
	"go/types"
)

func IsGnoBuiltin(obj types.Object) bool {
	if obj.Exported() {
		return false
	}

	switch obj.(type) {
	case *types.Func:
		if obj.Type().(*types.Signature).Recv() != nil {
			return false // method fn
		}

		switch obj.Name() {
		case "cross", "crossing":
			return true
		}
	}

	return false
}
