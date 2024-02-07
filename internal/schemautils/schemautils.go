package schemautils

import (
	"fmt"
	"strings"
)

// ConvertListToConcrete converts schema list, which is provided as []any to a slice of concrete type.
func ConvertListToConcrete[T any](list any) ([]T, error) {
	schemaList, ok := list.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T", list)
	}

	var out []T
	for _, listItem := range schemaList {
		listItemConcrete, ok := listItem.(T)
		if !ok {
			return nil, fmt.Errorf("unexpected type %T", listItem)
		}

		out = append(out, listItemConcrete)
	}

	return out, nil
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
