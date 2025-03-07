// Modified from original source github.com/mcuadros/go-lookup

/*
Small library on top of reflect for make lookups to Structs or Maps. Using a
very simple DSL you can access to any property, key or value of any value of Go.
*/
package lookup

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const (
	SplitToken     = "."
	IndexCloseChar = "]"
	IndexOpenChar  = "["
)

var (
	ErrMalformedIndex    = errors.New("malformed index key")
	ErrInvalidIndexUsage = errors.New("invalid index key usage")
	ErrKeyNotFound       = errors.New("unable to find the key")
)

// LookupString performs a lookup into a value, using a string. Same as `Lookup`
// but using a string with the keys separated by `.`
func LookupString(i any, path string, tags []string) (reflect.Value, error) {
	return Lookup(i, strings.Split(path, SplitToken), tags)
}

// LookupStringI is the same as LookupString, but the path is not case
// sensitive.
func LookupStringI(i any, path string, tags []string) (reflect.Value, error) {
	return LookupI(i, strings.Split(path, SplitToken), tags)
}

// Lookup performs a lookup into a value, using a path of keys. The key should
// match with a Field or a MapIndex.
// For slice you can use the syntax key[index] to access a specific index.
// If one key owns to a slice and an index is not
// specified the rest of the path will be applied to eval value of the
// slice, and the value will be merged into a slice.
func Lookup(i any, path []string, tags []string) (reflect.Value, error) {
	return lookup(i, false, path, tags)
}

// LookupI is the same as Lookup, but the path keys are not case sensitive.
func LookupI(i any, path []string, tags []string) (reflect.Value, error) {
	return lookup(i, true, path, tags)
}

func SetValueStringI[V int64 | int | string | bool | reflect.Value](i any, v V, path string, tags []string) error {
	// First find the variable
	dest, err := Lookup(i, strings.Split(path, SplitToken), tags)
	if err != nil {
		return fmt.Errorf("unable to set value %#v: %s", dest, err)
	}

	// can we set he value
	if !dest.CanSet() {
		return fmt.Errorf("unable to set value %#v at path %v", v, path)
	}

	var value reflect.Value
	var valueType reflect.Type

	switch any(v).(type) {
	case reflect.Value:
		valueType = any(v).(reflect.Value).Type()
		value = any(v).(reflect.Value)
	default:
		valueType = reflect.ValueOf(v).Type()
		value = reflect.ValueOf(v)
	}

	if dest.Type() != valueType {
		return fmt.Errorf("value of type %s is not assignable to type %s at path %s ", valueType.Name(), path, dest.Type().Name())
	}

	dest.Set(value)

	return nil
}

func lookup(i any, caseInsensitive bool, path []string, tags []string) (reflect.Value, error) {
	value := reflect.ValueOf(i)
	var parent reflect.Value
	var err error

	for i, part := range path {
		parent = value

		value, err = getValueByName(value, part, caseInsensitive, tags)
		if err == nil {
			continue
		}

		if !isAggregable(parent) {
			break
		}

		// if iterable and path contains index
		pp := path[i-1]
		if hasIndex(pp) {
			_, ind, _ := parseIndex(pp)
			parentLen := parent.Len()
			if ind >= parentLen {
				return reflect.Value{}, fmt.Errorf("requested index %d, is greater than object length %d", ind, parentLen)
			}

			value = parent.Index(ind)
			value, err = getValueByName(value, part, caseInsensitive, tags)
			break
		}

		value, err = aggreateAggregableValue(parent, path[i:], caseInsensitive, tags)

		break
	}

	return value, err
}

func getValueByName(v reflect.Value, key string, caseInsensitive bool, tags []string) (reflect.Value, error) {
	var value reflect.Value
	var index int = -1
	var err error

	key, index, err = parseIndex(key)
	if err != nil {
		return value, err
	}

	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		return getValueByName(v.Elem(), key, caseInsensitive, tags)
	case reflect.Struct:
		value = valueByNameOrTag(v, key, caseInsensitive, tags)

	case reflect.Map:
		kValue := reflect.Indirect(reflect.New(v.Type().Key()))
		kValue.SetString(key)
		value = v.MapIndex(kValue)
		if caseInsensitive && value.Kind() == reflect.Invalid {
			iter := v.MapRange()
			for iter.Next() {
				if strings.EqualFold(key, iter.Key().String()) {
					kValue.SetString(iter.Key().String())
					value = v.MapIndex(kValue)
					break
				}
			}
		}
	}

	if !value.IsValid() {
		return reflect.Value{}, ErrKeyNotFound
	}

	if index != -1 {
		if value.Kind() == reflect.Ptr {
			value = value.Elem()
		}

		if value.Type().Kind() != reflect.Slice {
			return reflect.Value{}, ErrInvalidIndexUsage
		}

		value = value.Index(index)
	}

	if value.Kind() == reflect.Ptr || value.Kind() == reflect.Interface {
		value = value.Elem()
	}

	return value, nil
}

func aggreateAggregableValue(v reflect.Value, path []string, caseInsensitive bool, tags []string) (reflect.Value, error) {
	values := make([]reflect.Value, 0)

	l := v.Len()
	if l == 0 {
		ty, ok := lookupType(v.Type(), path, caseInsensitive, tags)
		if !ok {
			return reflect.Value{}, ErrKeyNotFound
		}
		return reflect.MakeSlice(reflect.SliceOf(ty), 0, 0), nil
	}

	index := indexFunction(v)
	for i := 0; i < l; i++ {
		value, err := Lookup(index(i).Interface(), path, tags)
		if err != nil {
			return reflect.Value{}, err
		}

		values = append(values, value)
	}

	return mergeValue(values), nil
}

func indexFunction(v reflect.Value) func(i int) reflect.Value {
	switch v.Kind() {
	case reflect.Slice:
		return v.Index
	case reflect.Map:
		keys := v.MapKeys()
		return func(i int) reflect.Value {
			return v.MapIndex(keys[i])
		}
	default:
		panic("unsuported kind for index")
	}
}

func mergeValue(values []reflect.Value) reflect.Value {
	values = removeZeroValues(values)
	l := len(values)
	if l == 0 {
		return reflect.Value{}
	}

	sample := values[0]
	mergeable := isMergeable(sample)

	t := sample.Type()
	if mergeable {
		t = t.Elem()
	}

	value := reflect.MakeSlice(reflect.SliceOf(t), 0, 0)
	for i := 0; i < l; i++ {
		if !values[i].IsValid() {
			continue
		}

		if mergeable {
			value = reflect.AppendSlice(value, values[i])
		} else {
			value = reflect.Append(value, values[i])
		}
	}

	return value
}

func removeZeroValues(values []reflect.Value) []reflect.Value {
	l := len(values)

	var v []reflect.Value
	for i := 0; i < l; i++ {
		if values[i].IsValid() {
			v = append(v, values[i])
		}
	}

	return v
}

func isAggregable(v reflect.Value) bool {
	k := v.Kind()

	return k == reflect.Map || k == reflect.Slice
}

func isMergeable(v reflect.Value) bool {
	k := v.Kind()
	return k == reflect.Map || k == reflect.Slice
}

func hasIndex(s string) bool {
	return strings.Contains(s, IndexOpenChar)
}

func parseIndex(s string) (string, int, error) {
	start := strings.Index(s, IndexOpenChar)
	end := strings.Index(s, IndexCloseChar)

	if start == -1 && end == -1 {
		return s, -1, nil
	}

	if (start != -1 && end == -1) || (start == -1 && end != -1) {
		return "", -1, ErrMalformedIndex
	}

	index, err := strconv.Atoi(s[start+1 : end])
	if err != nil {
		return "", -1, ErrMalformedIndex
	}

	return s[:start], index, nil
}

func LookupType(ty reflect.Type, path []string, caseInsensitive bool, tags []string) (reflect.Type, bool) {
	return lookupType(ty, path, caseInsensitive, tags)
}

func lookupType(ty reflect.Type, path []string, caseInsensitive bool, tags []string) (reflect.Type, bool) {
	if len(path) == 0 {
		return ty, true
	}

	switch ty.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		if hasIndex(path[0]) {
			return lookupType(ty.Elem(), path[1:], caseInsensitive, tags)
		}
		// Aggregate.
		return lookupType(ty.Elem(), path, caseInsensitive, tags)
	case reflect.Ptr:
		return lookupType(ty.Elem(), path, caseInsensitive, tags)
	case reflect.Interface:
		// We can't know from here without a value. Let's just return this type.
		return ty, true
	case reflect.Struct:
		f := typeByNameOrTag(ty, path[0], caseInsensitive, tags)
		//fmt.Printf("Found struct kind: %v, path: %v %v tags: %v", f.Kind(), path, path[1:], tags)
		if f != nil {
			return lookupType(f, path[1:], caseInsensitive, tags)
		}
	}
	return nil, false
}

func valueByNameOrTag(v reflect.Value, key string, caseInsensitive bool, tags []string) reflect.Value {
	for i := 0; i < v.NumField(); i++ {
		if caseInsensitive {
			if strings.EqualFold(v.Type().Field(i).Name, key) {
				return v.Field(i)
			}
		} else {
			if v.Type().Field(i).Name == key {
				return v.Field(i)
			}
		}

		// if we do not find a value and we have tags specified
		if len(tags) > 0 {
			for _, t := range tags {
				tag := getTagValue(v.Type().Field(i), t)

				if caseInsensitive {
					if strings.EqualFold(key, tag) {
						return v.Field(i)
					}
				} else {
					if key == tag {
						return v.Field(i)
					}
				}
			}
		}
	}

	return reflect.Value{}
}

// getTag returns the first part from the given struct tag, this is often a referenced name
// i.e `json:"json_type_name,optional"`, getTag(field, "json") returns "json_type_name"
// returns "" if tag does not exist
func getTagValue(f reflect.StructField, tagName string) string {
	tag := string(f.Tag.Get(tagName))

	if tag != "" {
		parts := strings.Split(tag, ",")
		return parts[0]
	}

	return ""
}

func typeByNameOrTag(v reflect.Type, key string, caseInsensitive bool, tags []string) reflect.Type {
	for i := 0; i < v.NumField(); i++ {
		if caseInsensitive {
			if strings.EqualFold(v.Field(i).Name, key) {
				return v.Field(i).Type
			}
		} else {
			if v.Field(i).Name == key {
				return v.Field(i).Type
			}
		}

		// if we do not find a value and we have tags specified
		if len(tags) > 0 {
			for _, t := range tags {
				tag := getTagValue(v.Field(i), t)
				if caseInsensitive {
					if strings.EqualFold(tag, key) {
						return v.Field(i).Type
					}
				} else {
					if tag == key {
						return v.Field(i).Type
					}
				}
			}
		}
	}

	return nil
}
