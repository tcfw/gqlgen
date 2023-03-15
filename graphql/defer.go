package graphql

import (
	"context"
)

type (
	DeferedResolver struct {
		Label    string
		Path     []string
		Field    CollectedField
		Resolver func() Marshaler
	}
)

func GetPathFromFieldContext(ctx context.Context) []string {
	fc := GetFieldContext(ctx)
	if fc == nil {
		return nil
	}

	path := []string{}

	for parent := fc; parent != nil; parent = parent.Parent {

		if parent.Field.Field == nil {
			break
		}

		if parent.Field.Alias != "" {
			path = append(path, parent.Field.Alias)
		} else {
			path = append(path, parent.Field.Name)
		}
	}

	return path
}
