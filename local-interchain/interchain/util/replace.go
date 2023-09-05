package util

import (
	"reflect"
	"strings"
)

// Replaces all string values within a complex struct.
func ReplaceStringValues(data interface{}, oldStr, replacement string) {
	replaceStringFields(reflect.ValueOf(data), oldStr, replacement)
}

func replaceStringFields(value reflect.Value, oldStr, replacement string) {
	switch value.Kind() {
	case reflect.Ptr:
		if value.IsNil() {
			return
		}
		replaceStringFields(value.Elem(), oldStr, replacement)
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			replaceStringFields(field, oldStr, replacement)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			replaceStringFields(value.Index(i), oldStr, replacement)
		}
	case reflect.String:
		currentStr := value.String()
		if strings.Contains(currentStr, oldStr) {
			updatedStr := strings.Replace(currentStr, oldStr, replacement, -1)
			value.SetString(updatedStr)
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			replaceStringFields(value.MapIndex(key), oldStr, replacement)
		}
	}
}
