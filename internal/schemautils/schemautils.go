package schemautils

import (
	"fmt"
	"strings"
)

// ConvertListToConcrete converts schema list, which is provided as []any to a slice of concrete type.
func ConvertListToConcrete[T any](cidrBlocks any) ([]T, error) {
	cidrBlocksSlice, ok := cidrBlocks.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T", cidrBlocks)
	}

	var cidrList []T
	for _, cidrBlock := range cidrBlocksSlice {
		cidrBlockString, ok := cidrBlock.(T)
		if !ok {
			return nil, fmt.Errorf("unexpected type %T", cidrBlock)
		}

		cidrList = append(cidrList, cidrBlockString)
	}

	return cidrList, nil
}

func ConvertMapToConcrete[T any](mapVal map[string]interface{}) map[string]T {
	out := make(map[string]T, len(mapVal))
	for key, val := range mapVal {
		out[key] = val.(T)
	}
	return out
}

func ConvertMapFromConcrete[T any](mapVal map[string]T) map[string]interface{} {
	out := make(map[string]interface{}, len(mapVal))
	for key, val := range mapVal {
		out[key] = val
	}
	return out
}

func LowerCaseMapKeys[T any](mapVal map[string]T) map[string]T {
	out := make(map[string]T, len(mapVal))
	for key, val := range mapVal {
		out[strings.ToLower(key)] = val
	}
	return out
}
