package lookup

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/shipyard-run/hclconfig/test_fixtures/single_file/structs"
	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
	. "gopkg.in/check.v1"
)

func TestLookup_Map(t *testing.T) {
	value, err := Lookup(map[string]int{"foo": 42}, []string{"foo"}, nil)
	require.NoError(t, err)
	require.Equal(value.Int(), 42)
}

//func (s *S) TestLookup_Ptr(c *C) {
//	value, err := Lookup(&structFixture, []string{"String"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.String(), Equals, "foo")
//}
//
//func (s *S) TestLookup_Interface(c *C) {
//	value, err := Lookup(structFixture, []string{"Interface"}, nil)
//
//	c.Assert(err, IsNil)
//	c.Assert(value.String(), Equals, "foo")
//}
//
//func (s *S) TestLookup_StructBasic(c *C) {
//	value, err := Lookup(structFixture, []string{"String"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.String(), Equals, "foo")
//}
//
//func (s *S) TestLookup_StructJSONTagWithOptions(c *C) {
//	value, err := Lookup(structFixture, []string{"jsontagstring"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.String(), Equals, "foo")
//}
//
//func (s *S) TestLookup_StructHCLTagWithOptions(c *C) {
//	value, err := Lookup(structFixture, []string{"hcltagstring"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.String(), Equals, "foo")
//}
//
//func (s *S) TestLookup_StructJSONTag(c *C) {
//	value, err := Lookup(structFixture, []string{"jsontagmap", "foo"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Int(), Equals, int64(42))
//}
//
//func (s *S) TestLookup_StructHCLTag(c *C) {
//	value, err := Lookup(structFixture, []string{"hcltagmap", "foo"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Int(), Equals, int64(42))
//}
//
//func (s *S) TestLookup_StructPlusMap(c *C) {
//	value, err := Lookup(structFixture, []string{"Map", "foo"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Int(), Equals, int64(42))
//}
//
//func (s *S) TestLookup_MapNamed(c *C) {
//	value, err := Lookup(mapFixtureNamed, []string{"foo"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Int(), Equals, int64(42))
//}
//
//func (s *S) TestLookup_NotFound(c *C) {
//	_, err := Lookup(structFixture, []string{"qux"}, nil)
//	c.Assert(err, Equals, ErrKeyNotFound)
//
//	_, err = Lookup(mapFixture, []string{"qux"}, nil)
//	c.Assert(err, Equals, ErrKeyNotFound)
//}
//
//func (s *S) TestAggregableLookup_StructIndex(c *C) {
//	value, err := Lookup(structFixture, []string{"StructSlice", "Map", "foo"}, nil)
//
//	c.Assert(err, IsNil)
//	c.Assert(value.Interface(), DeepEquals, []int{42, 42})
//}
//
//func (s *S) TestAggregableLookup_StructEmpty(c *C) {
//	value, err := LookupString(emptyFixture, "volume.source", nil)
//
//	c.Assert(err, IsNil)
//	c.Assert(value.Interface(), DeepEquals, []string{})
//}
//
//func (s *S) TestAggregableLookup_StructNestedMap(c *C) {
//	value, err := Lookup(structFixture, []string{"StructSlice[0]", "String"}, nil)
//
//	c.Assert(err, IsNil)
//	c.Assert(value.Interface(), DeepEquals, "foo")
//}
//
//func (s *S) TestAggregableLookup_StructNested(c *C) {
//	value, err := Lookup(structFixture, []string{"StructSlice", "StructSlice", "String"}, nil)
//
//	c.Assert(err, IsNil)
//	c.Assert(value.Interface(), DeepEquals, []string{"bar", "foo", "qux", "baz"})
//}
//
//func (s *S) TestAggregableLookupString_Complex(c *C) {
//	value, err := LookupString(structFixture, "StructSlice.StructSlice[0].String", nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Interface(), DeepEquals, []string{"bar", "foo", "qux", "baz"})
//
//	value, err = LookupString(structFixture, "StructSlice[0].Map.foo", nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Interface(), DeepEquals, 42)
//
//	value, err = LookupString(mapComplexFixture, "map.bar", nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Interface(), DeepEquals, 1)
//
//	value, err = LookupString(mapComplexFixture, "list.baz", nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Interface(), DeepEquals, []int{1, 2, 3})
//}
//
//func (s *S) TestAggregableLookup_EmptySlice(c *C) {
//	fixture := [][]MyStruct{{}}
//	value, err := LookupString(fixture, "String", nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Interface().([]string), DeepEquals, []string{})
//}
//
//func (s *S) TestAggregableLookup_EmptyMap(c *C) {
//	fixture := map[string]*MyStruct{}
//	value, err := LookupString(fixture, "Map", nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Interface().([]map[string]int), DeepEquals, []map[string]int{})
//}
//
//func (s *S) TestMergeValue(c *C) {
//	v := mergeValue([]reflect.Value{reflect.ValueOf("qux"), reflect.ValueOf("foo")})
//	c.Assert(v.Interface(), DeepEquals, []string{"qux", "foo"})
//}
//
//func (s *S) TestMergeValueSlice(c *C) {
//	v := mergeValue([]reflect.Value{
//		reflect.ValueOf([]string{"foo", "bar"}),
//		reflect.ValueOf([]string{"qux", "baz"}),
//	})
//
//	c.Assert(v.Interface(), DeepEquals, []string{"foo", "bar", "qux", "baz"})
//}
//
//func (s *S) TestMergeValueZero(c *C) {
//	v := mergeValue([]reflect.Value{reflect.Value{}, reflect.ValueOf("foo")})
//	c.Assert(v.Interface(), DeepEquals, []string{"foo"})
//}
//
//func (s *S) TestParseIndex(c *C) {
//	key, index, err := parseIndex("foo[42]")
//	c.Assert(err, IsNil)
//	c.Assert(key, Equals, "foo")
//	c.Assert(index, Equals, 42)
//}
//
//func (s *S) TestParseIndexNooIndex(c *C) {
//	key, index, err := parseIndex("foo")
//	c.Assert(err, IsNil)
//	c.Assert(key, Equals, "foo")
//	c.Assert(index, Equals, -1)
//}
//
//func (s *S) TestParseIndexMalFormed(c *C) {
//	key, index, err := parseIndex("foo[]")
//	c.Assert(err, Equals, ErrMalformedIndex)
//	c.Assert(key, Equals, "")
//	c.Assert(index, Equals, -1)
//
//	key, index, err = parseIndex("foo[42")
//	c.Assert(err, Equals, ErrMalformedIndex)
//	c.Assert(key, Equals, "")
//	c.Assert(index, Equals, -1)
//
//	key, index, err = parseIndex("foo42]")
//	c.Assert(err, Equals, ErrMalformedIndex)
//	c.Assert(key, Equals, "")
//	c.Assert(index, Equals, -1)
//}
//
//func (s *S) TestLookup_CaseSensitive(c *C) {
//	_, err := Lookup(structFixture, []string{"STring"}, nil)
//	c.Assert(err, Equals, ErrKeyNotFound)
//}
//
//func (s *S) TestLookup_CaseInsensitive(c *C) {
//	value, err := LookupI(structFixture, []string{"STring"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.String(), Equals, "foo")
//}
//
//func (s *S) TestLookup_CaseInsensitive_ExactMatch(c *C) {
//	value, err := LookupI(caseFixtureStruct, []string{"Testfield"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Int(), Equals, int64(2))
//}
//
//func (s *S) TestLookup_CaseInsensitive_FirstMatch(c *C) {
//	value, err := LookupI(caseFixtureStruct, []string{"testfield"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Int(), Equals, int64(1))
//}
//
//func (s *S) TestLookup_CaseInsensitiveExactMatch(c *C) {
//	value, err := LookupI(structFixture, []string{"STring"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.String(), Equals, "foo")
//}
//
//func (s *S) TestLookup_Map_CaseSensitive(c *C) {
//	_, err := Lookup(map[string]int{"Foo": 42}, []string{"foo"}, nil)
//	c.Assert(err, Equals, ErrKeyNotFound)
//}
//
//func (s *S) TestLookup_Map_CaseInsensitive(c *C) {
//	value, err := LookupI(map[string]int{"Foo": 42}, []string{"foo"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Int(), Equals, int64(42))
//}
//
//func (s *S) TestLookup_Map_CaseInsensitive_ExactMatch(c *C) {
//	value, err := LookupI(caseFixtureMap, []string{"Testkey"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Int(), Equals, int64(2))
//}
//
//func (s *S) TestLookup_Map_CaseInsensitive_FirstMatch(c *C) {
//	value, err := LookupI(caseFixtureMap, []string{"testkey"}, nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.Int(), Equals, int64(1))
//}
//
//func (s *S) TestLookup_ListPtr(c *C) {
//	type Inner struct {
//		Value string
//	}
//
//	type Outer struct {
//		Values *[]Inner
//	}
//
//	values := []Inner{{Value: "first"}, {Value: "second"}}
//	data := Outer{Values: &values}
//
//	value, err := LookupStringI(data, "Values[0].Value", nil)
//	c.Assert(err, IsNil)
//	c.Assert(value.String(), Equals, "first")
//}
//
//func ExampleLookupString() {
//	type Cast struct {
//		Actor, Role string
//	}
//
//	type Serie struct {
//		Cast []Cast
//	}
//
//	series := map[string]Serie{
//		"A-Team": {Cast: []Cast{
//			{Actor: "George Peppard", Role: "Hannibal"},
//			{Actor: "Dwight Schultz", Role: "Murdock"},
//			{Actor: "Mr. T", Role: "Baracus"},
//			{Actor: "Dirk Benedict", Role: "Faceman"},
//		}},
//	}
//
//	q := "A-Team.Cast.Role"
//	value, _ := LookupString(series, q, nil)
//	fmt.Println(q, "->", value.Interface())
//
//	q = "A-Team.Cast[0].Actor"
//	value, _ = LookupString(series, q, nil)
//	fmt.Println(q, "->", value.Interface())
//
//	// Output:
//	// A-Team.Cast.Role -> [Hannibal Murdock Baracus Faceman]
//	// A-Team.Cast[0].Actor -> George Peppard
//}

func ExampleLookup() {
	type ExampleStruct struct {
		Values struct {
			Foo int
		}
	}

	i := ExampleStruct{}
	i.Values.Foo = 10

	value, _ := Lookup(i, []string{"Values", "Foo"}, nil)
	fmt.Println(value.Interface())
	// Output: 10
}

func ExampleCaseInsensitive() {
	type ExampleStruct struct {
		SoftwareUpdated bool
	}

	i := ExampleStruct{
		SoftwareUpdated: true,
	}

	value, _ := LookupStringI(i, "softwareupdated", nil)
	fmt.Println(value.Interface())
	// Output: true
}

func TestSomething(t *testing.T) {
	typ := reflect.TypeOf(structs.Container{})

	rt, err := lookupType(typ, []string{"volume", "source"}, false, []string{"hcl", "json"})
	require.NotNil(t, rt)
	assert.Nil(t, err)

	fmt.Printf("kind: %#v isSlice: %#v\n", rt.Name(), rt.Kind() == reflect.Slice)
}

type MyStruct struct {
	String      string         `json:"jsontagstring,omitempty" hcl:"hcltagstring,optional"`
	Map         map[string]int `json:"jsontagmap" hcl:"hcltagmap"`
	Nested      *MyStruct
	StructSlice []*MyStruct
	Interface   interface{}
	Slice       []string
}

type MyKey string

var mapFixtureNamed = map[MyKey]int{"foo": 42}
var mapFixture = map[string]int{"foo": 42}
var structFixture = MyStruct{
	String:    "foo",
	Map:       mapFixture,
	Interface: "foo",
	StructSlice: []*MyStruct{
		{Map: mapFixture, String: "foo", StructSlice: []*MyStruct{{String: "bar"}, {String: "foo"}}},
		{Map: mapFixture, String: "qux", StructSlice: []*MyStruct{{String: "qux"}, {String: "baz"}}},
	},
}

var emptyFixture = structs.Container{}

var mapComplexFixture = map[string]interface{}{
	"map": map[string]interface{}{
		"bar": 1,
	},
	"list": []map[string]interface{}{
		{"baz": 1},
		{"baz": 2},
		{"baz": 3},
	},
}

var caseFixtureStruct = struct {
	Foo       int
	TestField int
	Testfield int
	testField int
}{
	0, 1, 2, 3,
}

var caseFixtureMap = map[string]int{
	"Foo":     0,
	"TestKey": 1,
	"Testkey": 2,
	"testKey": 3,
}
