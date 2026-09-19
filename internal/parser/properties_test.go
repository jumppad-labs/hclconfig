package parser

import (
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/stretchr/testify/require"
)

// checkPropertyPath answers with the first path segment that does not exist, or
// an empty string when the path is acceptable. Every test below names one
// reference and one outcome, written out in full, so that the exact path under
// test and the exact answer expected are both visible without looking anywhere
// else.
//
// Accepting and rejecting are kept in separate functions throughout. The pair
// that matters most is `volume.*.destination`, which must be accepted, against
// `volume.*.destnation`, which must be rejected: that single difference is what
// separates a checker that catches a misspelling after a splat from one that
// rejects working configuration.

// Accepted paths.

func TestCheckPropertyPathAcceptsEmptyPath(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsMetaID(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "meta.id")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsMetaName(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "meta.name")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsDNS(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "dns")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsNetworkPositionID(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "network[0].id")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsNetworkPositionIPAddress(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "network[0].ip_address")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsSecondNetworkPositionName(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "network[1].name")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsResourcesCPUPin(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "resources.cpu_pin")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsResourcesMemory(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "resources.memory")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsResourcesUser(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "resources.user")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsVolumePositionSource(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "volume[0].source")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsVolumePositionDestination(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "volume[0].destination")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsVolumeDottedPositionSource(t *testing.T) {
	// the dotted spelling `volume.0.source` selects the same member as the
	// bracketed one and must be treated identically.
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "volume.0.source")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsVolumeSplatDestination(t *testing.T) {
	// selecting every volume at once and then naming a property those volumes
	// do have. This is the accepting half of the pair that matters most: it is
	// what a checker that stopped walking at the splat would still pass, and
	// what a checker that treated `*` as a property name would wrongly reject.
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "volume.*.destination")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsVolumeWithNoMemberSelected(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "volume")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsCreatedNetwork(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "created_network")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsCreatedNetworkSplatName(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "created_network.*.name")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsCreatedNetworkMap(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "created_network_map")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsCreatedNetworkMapKeyThenSubnet(t *testing.T) {
	// `one` is a map key, written exactly as a property name would be. It is
	// recognisable as a key only by the map it stands against, which is why any
	// segment standing against a collection selects a member rather than naming
	// a property on the collection itself.
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "created_network_map.one.subnet")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsCreatedNetworkMapSplatThenSubnet(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "created_network_map.*.subnet")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsCreatedNetworkMapKeyThenNestedMetaName(t *testing.T) {
	// the walk continues past the selected member and through a property of its
	// own, so `meta.name` is checked against the Network the map holds.
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "created_network_map.one.meta.name")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsEnvKey(t *testing.T) {
	// env is a map of strings. Which member is meant no longer matters once the
	// members are scalars, so the walk stops and accepts rather than judging an
	// arbitrary key against a property list.
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "env.MY_VAR")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsEnvWithNoKeySelected(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "env")

	require.Equal(t, "", missing)
}

// Accepted paths where the type stops being knowable.

func TestCheckPropertyPathAcceptsAnyPropertyBeneathAnOutput(t *testing.T) {
	// a reference to an output names the value the output holds, not a field of
	// the Output declaration. That value's shape is decided while the
	// configuration is applied, so nothing beneath it can be judged now. This is
	// the shape `module.consul_1.output.combined_map.name` takes.
	missing := checkPropertyPath(reflect.TypeOf(resources.Output{}), "combined_map.name")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsPropertyThatIsNotAFieldOfOutput(t *testing.T) {
	// `nosuch` is not a field of the Output struct at all. It must still be
	// accepted, because what is being named is the held value rather than the
	// declaration.
	missing := checkPropertyPath(reflect.TypeOf(resources.Output{}), "nosuch")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsAnyPropertyBeneathAVariable(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(resources.Variable{}), "nosuch.deeper")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsPropertyBeneathACtyValue(t *testing.T) {
	// Template.Vars is a cty.Value, which carries whatever the configuration
	// puts in it. Nothing beneath it is knowable before the configuration is
	// applied, so the remainder of the path is accepted.
	missing := checkPropertyPath(reflect.TypeOf(structs.Template{}), "vars.data_dir")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsMemberSelectedFromACtyValue(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Template{}), "vars.*.anything")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsMemberSelectedFromACollectionOfScalars(t *testing.T) {
	// dns is a list of strings. Selecting a position gives a scalar, beneath
	// which nothing can be named, so the walk stops and accepts.
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "dns.0.anything")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsPropertyBeneathAScalarMapMember(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "env.MY_VAR.anything")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsPropertyBeneathATypeWithNoDescribedProperties(t *testing.T) {
	// a type that describes no properties of its own says nothing about what a
	// path beneath it may name, so nothing is rejected on the strength of an
	// incomplete description.
	type undescribed struct {
		Hidden string
	}

	missing := checkPropertyPath(reflect.TypeOf(undescribed{}), "anything.at.all")

	require.Equal(t, "", missing)
}

func TestCheckPropertyPathAcceptsPropertyBeneathAnInterface(t *testing.T) {
	// an interface carries anything at all, so it cannot say what properties it
	// has until it holds something.
	type holder struct {
		Value any `xcl:"value,optional"`
	}

	missing := checkPropertyPath(reflect.TypeOf(holder{}), "value.anything")

	require.Equal(t, "", missing)
}

// Rejected paths.

func TestCheckPropertyPathRejectsVolumeSplatMisspelledDestination(t *testing.T) {
	// the rejecting half of the pair that matters most. `destnation` is a
	// misspelling of a real property of a volume, named after selecting every
	// volume at once, and a checker that stopped walking at the splat would let
	// it through silently.
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "volume.*.destnation")

	require.Equal(t, "destnation", missing)
}

func TestCheckPropertyPathRejectsVolumeDottedPositionMisspelledDestination(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "volume.0.destnation")

	require.Equal(t, "destnation", missing)
}

func TestCheckPropertyPathRejectsVolumePositionMisspelledDestination(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "volume[0].destnation")

	require.Equal(t, "destnation", missing)
}

func TestCheckPropertyPathRejectsCreatedNetworkMapKeyThenMisspelledSubnet(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "created_network_map.one.subnett")

	require.Equal(t, "subnett", missing)
}

func TestCheckPropertyPathRejectsCreatedNetworkMapSplatThenMisspelledSubnet(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "created_network_map.*.subnett")

	require.Equal(t, "subnett", missing)
}

func TestCheckPropertyPathRejectsPropertyTheResourceDoesNotHave(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "nosuchprop")

	require.Equal(t, "nosuchprop", missing)
}

func TestCheckPropertyPathRejectsPropertyNestedResourcesDoesNotHave(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "resources.nosuch")

	require.Equal(t, "nosuch", missing)
}

func TestCheckPropertyPathRejectsMisspelledMetaName(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "meta.nam")

	require.Equal(t, "nam", missing)
}

func TestCheckPropertyPathRejectsMetaPropertiesWhichCarriesNoHCLTag(t *testing.T) {
	// Meta.Properties is a real Go field, but it carries only a json tag and is
	// therefore invisible to configuration. Naming it is naming something that
	// does not exist as far as a configuration author is concerned.
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "meta.properties")

	require.Equal(t, "properties", missing)
}

func TestCheckPropertyPathRejectsMetaLinksWhichCarriesNoHCLTag(t *testing.T) {
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "meta.links")

	require.Equal(t, "links", missing)
}

func TestCheckPropertyPathReportsOnlyTheFirstUnknownSegment(t *testing.T) {
	// once a segment is unknown, the type beyond it is unknown too, so later
	// segments cannot be judged. Only the first is named.
	missing := checkPropertyPath(reflect.TypeOf(structs.Container{}), "nosuchprop.alsomissing")

	require.Equal(t, "nosuchprop", missing)
}

// propertyProblem renders the message an author reads, so its exact wording is
// pinned here rather than only where it is emitted.

func TestPropertyProblemNamesTheReferenceAndTheMissingProperty(t *testing.T) {
	message := propertyProblem("resource.container.consul.meta.nam", "nam")

	require.Equal(t, "reference 'resource.container.consul.meta.nam' names property 'nam', which does not exist", message)
}
