package parser

import (
	"reflect"
	"sort"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"

	// The import path is upstream go-cty, but go.mod replaces it with the
	// jumppad-labs fork. The fork reads the `hcl` struct tag where upstream
	// reads `cty`, and it is the fork that the runtime uses when a
	// configuration is applied.
	"github.com/zclconf/go-cty/cty/gocty"
)

// sortedNames returns the keys of a property set in a stable order so a failure
// message reads the same way every run.
func sortedNames(properties map[string]reflect.Type) []string {
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// -----------------------------------------------------------------------------
// Rule-level cases, against structs defined here so a change to a production
// type cannot quietly change what these tests are asserting.
// -----------------------------------------------------------------------------

type plainStruct struct {
	Subnet string `hcl:"subnet,optional"`
	Name   string `hcl:"name"`
}

func TestPropertyNamesReadsNameBeforeFirstComma(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(plainStruct{}))

	require.Equal(t, []string{"name", "subnet"}, sortedNames(properties))
}

func TestPropertyNamesRecordsFieldTypeAgainstName(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(plainStruct{}))

	require.Equal(t, reflect.TypeOf(""), properties["subnet"])
	require.Equal(t, reflect.TypeOf(""), properties["name"])
}

type untaggedFieldStruct struct {
	Tagged   string `hcl:"tagged,optional"`
	Untagged string
	JSONOnly string `json:"json_only"`
}

func TestPropertyNamesIncludesTaggedField(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(untaggedFieldStruct{}))

	require.Equal(t, []string{"tagged"}, sortedNames(properties))
}

func TestPropertyNamesOmitsFieldWithNoHCLTag(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(untaggedFieldStruct{}))

	require.NotContains(t, properties, "Untagged")
	require.NotContains(t, properties, "untagged")
	require.NotContains(t, properties, "JSONOnly")
	require.NotContains(t, properties, "json_only")
}

type embeddedInner struct {
	InnerOne string `hcl:"inner_one,optional"`
	InnerTwo string `hcl:"inner_two,optional"`
}

type anonymousRemainStruct struct {
	embeddedInner `hcl:",remain"`

	Own string `hcl:"own,optional"`
}

func TestPropertyNamesFlattensEmbeddedProperties(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(anonymousRemainStruct{}))

	require.Equal(t, []string{"inner_one", "inner_two", "own"}, sortedNames(properties))
}

func TestPropertyNamesEmptyFirstSegmentContributesNoName(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(anonymousRemainStruct{}))

	// `hcl:",remain"` flattens but names nothing of its own, so neither the
	// empty string nor the Go field name is addressable.
	require.NotContains(t, properties, "")
	require.NotContains(t, properties, "embeddedInner")
}

type namedRemainStruct struct {
	embeddedInner `hcl:"rm,remain"`

	Own string `hcl:"own,optional"`
}

func TestPropertyNamesNamedEmbeddedIsReachableByItsOwnName(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(namedRemainStruct{}))

	require.Contains(t, properties, "rm")
	require.Equal(t, reflect.TypeOf(embeddedInner{}), properties["rm"])
}

func TestPropertyNamesNamedEmbeddedAlsoFlattens(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(namedRemainStruct{}))

	// `hcl:"rm,remain"` does both: it registers `rm` and flattens the
	// embedded properties into the containing type.
	require.Equal(t, []string{"inner_one", "inner_two", "own", "rm"}, sortedNames(properties))
}

type embeddedMiddle struct {
	embeddedInner `hcl:",remain"`

	Middle string `hcl:"middle,optional"`
}

type embeddedOuter struct {
	embeddedMiddle `hcl:",remain"`

	Outer string `hcl:"outer,optional"`
}

func TestPropertyNamesFlattensEmbeddingRecursively(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(embeddedOuter{}))

	require.Equal(t, []string{"inner_one", "inner_two", "middle", "outer"}, sortedNames(properties))
}

type pointerEmbeddedStruct struct {
	*embeddedInner `hcl:"ptr,remain"`

	Own string `hcl:"own,optional"`
}

func TestPropertyNamesDereferencesPointerEmbeddedField(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(pointerEmbeddedStruct{}))

	require.Equal(t, []string{"inner_one", "inner_two", "own", "ptr"}, sortedNames(properties))
}

func TestPropertyNamesAcceptsPointerToStruct(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(&plainStruct{}))

	require.Equal(t, []string{"name", "subnet"}, sortedNames(properties))
}

type selfEmbeddingStruct struct {
	*selfEmbeddingStruct `hcl:"self,remain"`

	Own string `hcl:"own,optional"`
}

func TestPropertyNamesTerminatesOnSelfEmbeddingType(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(selfEmbeddingStruct{}))

	require.Equal(t, []string{"own", "self"}, sortedNames(properties))
}

func TestPropertyNamesReturnsEmptyForNonStruct(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(""))

	require.Empty(t, properties)
}

// -----------------------------------------------------------------------------
// Production types. These duplicate some of the rule-level coverage on purpose:
// they catch a drift in either propertyNames or the types themselves.
// -----------------------------------------------------------------------------

func TestPropertyNamesForMeta(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(types.Meta{}))

	require.Equal(t,
		[]string{"column", "file", "id", "line", "module", "name", "type"},
		sortedNames(properties),
	)
}

func TestPropertyNamesForMetaOmitsFieldsCarryingOnlyJSONTags(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(types.Meta{}))

	// Properties, Links and Status are real Go fields on types.Meta but carry
	// only `json` tags, so configuration cannot address them at all.
	require.NotContains(t, properties, "properties")
	require.NotContains(t, properties, "Properties")
	require.NotContains(t, properties, "links")
	require.NotContains(t, properties, "Links")
	require.NotContains(t, properties, "status")
	require.NotContains(t, properties, "Status")
}

func TestPropertyNamesForResourceBase(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(types.ResourceBase{}))

	require.Equal(t, []string{"depends_on", "disabled", "meta"}, sortedNames(properties))
}

func TestPropertyNamesForNetwork(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(structs.Network{}))

	require.Equal(t, []string{"depends_on", "disabled", "meta", "subnet"}, sortedNames(properties))
}

func TestPropertyNamesForNetworkFlattensResourceBaseWithoutNamingIt(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(structs.Network{}))

	// structs.Network embeds types.ResourceBase as `hcl:",remain"`, so its
	// properties flatten in but the embed itself contributes no name.
	require.Contains(t, properties, "depends_on")
	require.Contains(t, properties, "disabled")
	require.Contains(t, properties, "meta")
	require.NotContains(t, properties, "rm")
	require.NotContains(t, properties, "ResourceBase")
}

func TestPropertyNamesForTemplate(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(structs.Template{}))

	require.Equal(t,
		[]string{"append_file", "depends_on", "destination", "disabled", "inner", "meta", "source", "vars"},
		sortedNames(properties),
	)
}

func TestPropertyNamesForContainer(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(structs.Container{}))

	require.Equal(t,
		[]string{
			"build",
			"command",
			"container_id",
			"created_network",
			"created_network_map",
			"default",
			"depends_on",
			"disabled",
			"dns",
			"entrypoint",
			"env",
			"max_restart_count",
			"meta",
			"network",
			"networkobj",
			"port",
			"privileged",
			"resources",
			"rm",
			"run_as",
			"volume",
		},
		sortedNames(properties),
	)
}

func TestPropertyNamesForContainerReachesNamedEmbedBothWays(t *testing.T) {
	properties := propertyNames(reflect.TypeOf(structs.Container{}))

	// structs.ContainerBase embeds types.ResourceBase as `hcl:"rm,remain"`, so
	// the same embed is reachable under its own name and through its flattened
	// properties. Container reaches both through a second, unnamed embed of
	// ContainerBase.
	require.Contains(t, properties, "rm")
	require.Equal(t, reflect.TypeOf(types.ResourceBase{}), properties["rm"])

	require.Contains(t, properties, "depends_on")
	require.Contains(t, properties, "disabled")
	require.Contains(t, properties, "meta")
}

// -----------------------------------------------------------------------------
// Plugin-supplied types. internal/schema/deserialize.go rebuilds a plugin's
// types as anonymous reflect.StructOf structs, reconstructing the original
// `hcl` tags verbatim and setting Anonymous on embedded fields.
// -----------------------------------------------------------------------------

// pluginStyleType builds an anonymous struct the way
// internal/schema/deserialize.go does: named fields carrying reconstructed
// `hcl` tags, plus an anonymous embedded field tagged `hcl:"rm,remain"`.
func pluginStyleType() reflect.Type {
	return reflect.StructOf([]reflect.StructField{
		{
			Name:      "ResourceBase",
			Type:      reflect.TypeOf(types.ResourceBase{}),
			Tag:       reflect.StructTag(`hcl:"rm,remain" json:"resource_base,omitempty"`),
			Anonymous: true,
		},
		{
			Name: "Subnet",
			Type: reflect.TypeOf(""),
			Tag:  reflect.StructTag(`hcl:"subnet,optional" json:"subnet,omitempty"`),
		},
		{
			Name: "Ports",
			Type: reflect.SliceOf(reflect.TypeOf(0)),
			Tag:  reflect.StructTag(`hcl:"port,block" json:"port,omitempty"`),
		},
	})
}

func TestPropertyNamesForPluginSuppliedType(t *testing.T) {
	properties := propertyNames(pluginStyleType())

	require.Equal(t,
		[]string{"depends_on", "disabled", "meta", "port", "rm", "subnet"},
		sortedNames(properties),
	)
}

func TestPropertyNamesForPluginSuppliedTypeMatchesEquivalentNamedType(t *testing.T) {
	// A plugin type rebuilt by reflect.StructOf and a named Go type carrying
	// the same tags must be answered identically.
	fromPlugin := propertyNames(pluginStyleType())
	fromNamed := propertyNames(reflect.TypeOf(namedRemainEquivalent{}))

	require.Equal(t, sortedNames(fromNamed), sortedNames(fromPlugin))
}

// namedRemainEquivalent is the named-type twin of pluginStyleType.
type namedRemainEquivalent struct {
	types.ResourceBase `hcl:"rm,remain" json:"resource_base,omitempty"`

	Subnet string `hcl:"subnet,optional" json:"subnet,omitempty"`
	Ports  []int  `hcl:"port,block" json:"port,omitempty"`
}

// -----------------------------------------------------------------------------
// Cross-check against the runtime.
//
// This is the phase's central acceptance criterion made executable: the names
// propertyNames reports must match exactly the attributes the forked go-cty
// derives for the same type, which is what the system accepts when a
// configuration is applied. Asserted in both directions so neither an extra
// nor a missing name can pass.
// -----------------------------------------------------------------------------

// requireMatchesImpliedType fails unless propertyNames and gocty.ImpliedType
// agree on the complete set of names for value, naming the offenders on either
// side when they do not.
func requireMatchesImpliedType(t *testing.T, value any) {
	t.Helper()

	ours := propertyNames(reflect.TypeOf(value))

	ctyType, err := gocty.ImpliedType(value)
	require.NoError(t, err, "gocty could not derive a type for %T", value)
	require.True(t, ctyType.IsObjectType(), "gocty derived a non-object type for %T", value)

	theirs := ctyType.AttributeTypes()

	onlyOurs := []string{}
	for name := range ours {
		if _, ok := theirs[name]; !ok {
			onlyOurs = append(onlyOurs, name)
		}
	}
	sort.Strings(onlyOurs)

	onlyTheirs := []string{}
	for name := range theirs {
		if _, ok := ours[name]; !ok {
			onlyTheirs = append(onlyTheirs, name)
		}
	}
	sort.Strings(onlyTheirs)

	require.Empty(t, onlyOurs,
		"propertyNames reports properties for %T that the runtime does not accept: %v", value, onlyOurs)
	require.Empty(t, onlyTheirs,
		"the runtime accepts properties for %T that propertyNames does not report: %v", value, onlyTheirs)

	require.Equal(t, len(theirs), len(ours),
		"propertyNames and the runtime disagree on how many properties %T has", value)
}

func TestPropertyNamesForContainerMatchesRuntimeImpliedType(t *testing.T) {
	requireMatchesImpliedType(t, structs.Container{})
}

func TestPropertyNamesForNetworkMatchesRuntimeImpliedType(t *testing.T) {
	requireMatchesImpliedType(t, structs.Network{})
}

func TestPropertyNamesForTemplateMatchesRuntimeImpliedType(t *testing.T) {
	requireMatchesImpliedType(t, structs.Template{})
}

func TestPropertyNamesForResourceBaseMatchesRuntimeImpliedType(t *testing.T) {
	requireMatchesImpliedType(t, types.ResourceBase{})
}

func TestPropertyNamesForMetaMatchesRuntimeImpliedType(t *testing.T) {
	requireMatchesImpliedType(t, types.Meta{})
}
