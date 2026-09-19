// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package gohcl

import (
	"testing"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/stretchr/testify/require"
)

type checkBase struct {
	DependsOn []string `xcl:"depends_on,optional"`
}

type checkTimeouts struct {
	Connect int `xcl:"connect"`
}

type checkDatabase struct {
	checkBase `xcl:",remain"`

	Location string         `xcl:"location"`
	Port     int            `xcl:"port,optional"`
	Timeouts *checkTimeouts `xcl:"timeouts,block"`
}

type checkNoRemain struct {
	Location string `xcl:"location"`
}

type checkBodyRemain struct {
	Location string   `xcl:"location"`
	Rest     hcl.Body `xcl:",remain"`
}

func parseCheckBody(t *testing.T, src string) hcl.Body {
	t.Helper()

	f, diags := hclsyntax.ParseConfig([]byte(src), "test.xcl", hcl.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors(), diags.Error())

	return f.Body
}

func TestCheckBodyAcceptsKnownAttributes(t *testing.T) {
	body := parseCheckBody(t, `
location = "eu-west"
port     = 5432
`)

	diags := CheckBody(body, &checkDatabase{})
	require.False(t, diags.HasErrors(), diags.Error())
}

func TestCheckBodyRejectsUnknownAttribute(t *testing.T) {
	body := parseCheckBody(t, `
location = "eu-west"
colour   = "red"
`)

	diags := CheckBody(body, &checkDatabase{})
	require.True(t, diags.HasErrors())
	require.Len(t, diags, 1)
	require.Equal(t, "Unsupported argument", diags[0].Summary)
	require.Contains(t, diags[0].Detail, `"colour"`)
	require.Equal(t, 3, diags[0].Subject.Start.Line)
}

func TestCheckBodyRejectsUnknownBlock(t *testing.T) {
	body := parseCheckBody(t, `
location = "eu-west"
backups {}
`)

	diags := CheckBody(body, &checkDatabase{})
	require.True(t, diags.HasErrors())
	require.Len(t, diags, 1)
	require.Equal(t, "Unsupported block type", diags[0].Summary)
	require.Contains(t, diags[0].Detail, `"backups"`)
}

func TestCheckBodyAcceptsKnownNestedBlock(t *testing.T) {
	body := parseCheckBody(t, `
location = "eu-west"
timeouts {
  connect = 30
}
`)

	diags := CheckBody(body, &checkDatabase{})
	require.False(t, diags.HasErrors(), diags.Error())
}

func TestCheckBodyRejectsUnknownAttributeInNestedBlock(t *testing.T) {
	body := parseCheckBody(t, `
location = "eu-west"
timeouts {
  connect = 30
  retries = 3
}
`)

	diags := CheckBody(body, &checkDatabase{})
	require.True(t, diags.HasErrors())
	require.Len(t, diags, 1)
	require.Equal(t, "Unsupported argument", diags[0].Summary)
	require.Contains(t, diags[0].Detail, `"retries"`)
	require.Equal(t, 5, diags[0].Subject.Start.Line)
}

func TestCheckBodyAcceptsAttributesOfEmbeddedRemainStruct(t *testing.T) {
	body := parseCheckBody(t, `
location   = "eu-west"
depends_on = ["resource.database.other"]
`)

	diags := CheckBody(body, &checkDatabase{})
	require.False(t, diags.HasErrors(), diags.Error())
}

func TestCheckBodyRejectsUnknownAttributeWhenRemainStructPresent(t *testing.T) {
	body := parseCheckBody(t, `
location = "eu-west"
bogus    = 1
`)

	diags := CheckBody(body, &checkDatabase{})
	require.True(t, diags.HasErrors())
	require.Len(t, diags, 1)
	require.Contains(t, diags[0].Detail, `"bogus"`)
}

func TestCheckBodyRejectsUnknownAttributeWithoutRemain(t *testing.T) {
	body := parseCheckBody(t, `
location = "eu-west"
bogus    = 1
`)

	diags := CheckBody(body, &checkNoRemain{})
	require.True(t, diags.HasErrors())
	require.Len(t, diags, 1)
	require.Contains(t, diags[0].Detail, `"bogus"`)
}

func TestCheckBodyAcceptsAnythingInRemainBody(t *testing.T) {
	body := parseCheckBody(t, `
location = "eu-west"
bogus    = 1
extra {}
`)

	diags := CheckBody(body, &checkBodyRemain{})
	require.False(t, diags.HasErrors(), diags.Error())
}

func TestCheckBodyIgnoresMissingRequiredAttribute(t *testing.T) {
	body := parseCheckBody(t, `
port = 5432
`)

	diags := CheckBody(body, &checkDatabase{})
	require.False(t, diags.HasErrors(), diags.Error())
}

func TestCheckBodyDoesNotEvaluateExpressions(t *testing.T) {
	body := parseCheckBody(t, `
location = resource.database.missing.location
port     = var.undefined + 1
`)

	diags := CheckBody(body, &checkDatabase{})
	require.False(t, diags.HasErrors(), diags.Error())
}
