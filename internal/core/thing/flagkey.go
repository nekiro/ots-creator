package thing

import (
	"reflect"
	"strings"
	"sync"
)

var (
	flagFieldsOnce sync.Once
	flagFields     map[string]int
)

// FlagByKey reports a boolean property by its JSON name. ok is false when no
// boolean property has that name.
func (p *Properties) FlagByKey(key string) (value, ok bool) {
	flagFieldsOnce.Do(func() {
		flagFields = map[string]int{}
		propertiesType := reflect.TypeOf(Properties{})
		for i := range propertiesType.NumField() {
			field := propertiesType.Field(i)
			if field.Type.Kind() != reflect.Bool {
				continue
			}
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name != "" {
				flagFields[name] = i
			}
		}
	})
	i, ok := flagFields[key]
	if !ok {
		return false, false
	}
	return reflect.ValueOf(p).Elem().Field(i).Bool(), true
}
