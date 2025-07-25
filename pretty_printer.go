package hclconfig

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/mitchellh/go-wordwrap"
)

// PrintFormat represents different output formats for the pretty printer
type PrintFormat string

const (
	FormatTable PrintFormat = "table"
	FormatTree  PrintFormat = "tree"
	FormatCard  PrintFormat = "card"
	FormatJSON  PrintFormat = "json"
)

// PrinterOptions configures the ResourcePrinter behavior
type PrinterOptions struct {
	// Output writer (defaults to os.Stdout)
	Writer io.Writer
	// Enable/disable color output (auto-detected by default)
	ColorEnabled *bool
	// Maximum width for text wrapping
	MaxWidth int
	// Show all fields including empty ones
	ShowEmpty bool
	// Verbosity level (0=basic, 1=detailed, 2=all)
	Verbosity int
}

// PrinterOption is a functional option for configuring the printer
type PrinterOption func(*PrinterOptions)

// WithWriter sets the output writer
func WithWriter(w io.Writer) PrinterOption {
	return func(o *PrinterOptions) {
		o.Writer = w
	}
}

// WithColor enables or disables color output
func WithColor(enabled bool) PrinterOption {
	return func(o *PrinterOptions) {
		o.ColorEnabled = &enabled
	}
}

// WithMaxWidth sets the maximum width for text wrapping
func WithMaxWidth(width int) PrinterOption {
	return func(o *PrinterOptions) {
		o.MaxWidth = width
	}
}

// WithShowEmpty shows all fields including empty ones
func WithShowEmpty(show bool) PrinterOption {
	return func(o *PrinterOptions) {
		o.ShowEmpty = show
	}
}

// WithVerbosity sets the verbosity level
func WithVerbosity(level int) PrinterOption {
	return func(o *PrinterOptions) {
		o.Verbosity = level
	}
}

// ResourcePrinter handles pretty printing of resources
type ResourcePrinter struct {
	options PrinterOptions
	colors  struct {
		created color.Attribute
		pending color.Attribute
		failed  color.Attribute
		header  color.Attribute
		field   color.Attribute
		value   color.Attribute
	}
}

// NewResourcePrinter creates a new ResourcePrinter with the given options
func NewResourcePrinter(opts ...PrinterOption) *ResourcePrinter {
	options := PrinterOptions{
		Writer:    os.Stdout,
		MaxWidth:  80,
		ShowEmpty: false,
		Verbosity: 1,
	}

	for _, opt := range opts {
		opt(&options)
	}

	// Auto-detect color support if not explicitly set
	if options.ColorEnabled == nil {
		enabled := color.NoColor == false
		options.ColorEnabled = &enabled
	}

	printer := &ResourcePrinter{
		options: options,
	}

	// Configure colors
	if *options.ColorEnabled {
		printer.colors.created = color.FgGreen
		printer.colors.pending = color.FgYellow
		printer.colors.failed = color.FgRed
		printer.colors.header = color.FgCyan
		printer.colors.field = color.FgBlue
		printer.colors.value = color.FgWhite
	} else {
		// Disable all colors
		color.NoColor = true
	}

	return printer
}

// PrintResource prints a single resource in the specified format
func (p *ResourcePrinter) PrintResource(resource any, format PrintFormat) error {
	switch format {
	case FormatTable:
		return p.printTable(resource)
	case FormatTree:
		return p.printTree(resource)
	case FormatCard:
		return p.printCard(resource)
	case FormatJSON:
		return p.printJSON(resource)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// PrintResources prints multiple resources in the specified format
func (p *ResourcePrinter) PrintResources(resources []any, format PrintFormat) error {
	for i, resource := range resources {
		if i > 0 {
			fmt.Fprintln(p.options.Writer) // Add spacing between resources
		}
		if err := p.PrintResource(resource, format); err != nil {
			return err
		}
	}
	return nil
}

// getStatusColor returns the appropriate color for a resource status
func (p *ResourcePrinter) getStatusColor(status string) color.Attribute {
	switch status {
	case "created":
		return p.colors.created
	case "pending":
		return p.colors.pending
	case "failed":
		return p.colors.failed
	default:
		return p.colors.value
	}
}

// formatValue formats a value for display, handling different types appropriately
func (p *ResourcePrinter) formatValue(value interface{}) string {
	if value == nil {
		return ""
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		str := v.String()
		if len(str) > p.options.MaxWidth-20 { // Leave room for field names
			str = wordwrap.WrapString(str, uint(p.options.MaxWidth-20))
		}
		return str
	case reflect.Slice, reflect.Array:
		if v.Len() == 0 && !p.options.ShowEmpty {
			return ""
		}
		var items []string
		for i := 0; i < v.Len(); i++ {
			items = append(items, fmt.Sprintf("%v", v.Index(i).Interface()))
		}
		return fmt.Sprintf("[%s]", strings.Join(items, ", "))
	case reflect.Map:
		if v.Len() == 0 && !p.options.ShowEmpty {
			return ""
		}
		var items []string
		for _, key := range v.MapKeys() {
			val := v.MapIndex(key)
			items = append(items, fmt.Sprintf("%v: %v", key.Interface(), val.Interface()))
		}
		sort.Strings(items) // Sort for consistent output
		return fmt.Sprintf("{%s}", strings.Join(items, ", "))
	case reflect.Struct:
		return fmt.Sprintf("<%s>", v.Type().Name())
	case reflect.Ptr:
		if v.IsNil() {
			return ""
		}
		return p.formatValue(v.Elem().Interface())
	default:
		return fmt.Sprintf("%v", value)
	}
}

// visualLength returns the visual length of a string, excluding ANSI escape codes
func visualLength(s string) int {
	// Remove ANSI escape codes for accurate length calculation
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	cleaned := ansiRegex.ReplaceAllString(s, "")
	return len(cleaned)
}

// shouldShowField determines if a field should be displayed based on options
func (p *ResourcePrinter) shouldShowField(value interface{}) bool {
	if p.options.ShowEmpty {
		return true
	}

	if value == nil {
		return false
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return v.String() != ""
	case reflect.Slice, reflect.Array:
		return v.Len() > 0
	case reflect.Map:
		return v.Len() > 0
	case reflect.Ptr:
		return !v.IsNil()
	default:
		return true
	}
}

// printTable prints a resource in table format with borders
func (p *ResourcePrinter) printTable(resource any) error {
	meta, err := types.GetMeta(resource)
	if err != nil {
		return fmt.Errorf("resource does not have ResourceBase: %w", err)
	}

	// Calculate the width for the table
	width := p.options.MaxWidth
	if width < 40 {
		width = 40
	}

	// Create header
	header := fmt.Sprintf("Resource: %s", meta.ID)
	if len(header) > width-4 {
		header = header[:width-7] + "..."
	}

	// Print top border
	fmt.Fprintf(p.options.Writer, "┌%s┐\n", strings.Repeat("─", width-2))

	// Print header
	headerColored := color.New(p.colors.header).Sprint(header)
	headerVisualLen := visualLength(headerColored)
	padding := width - headerVisualLen - 4 // "│ " + header + " │"
	if padding < 0 {
		padding = 0
	}
	fmt.Fprintf(p.options.Writer, "│ %s%s │\n",
		headerColored,
		strings.Repeat(" ", padding))

	// Print separator
	fmt.Fprintf(p.options.Writer, "├%s┤\n", strings.Repeat("─", width-2))

	// Print metadata fields
	fields := []struct {
		name  string
		value interface{}
	}{
		{"Type", meta.Type},
		{"Status", meta.Status},
		{"File", fmt.Sprintf("%s:%d", meta.File, meta.Line)},
		{"Module", meta.Module},
	}

	// Add dependencies if they exist
	deps, err := types.GetDependencies(resource)
	if err != nil {
		deps = []string{} // Use empty slice if unable to get dependencies
	}
	if len(deps) > 0 || p.options.ShowEmpty {
		fields = append(fields, struct {
			name  string
			value interface{}
		}{"Dependencies", deps})
	}

	// Add links if they exist
	if len(meta.Links) > 0 || p.options.ShowEmpty {
		fields = append(fields, struct {
			name  string
			value interface{}
		}{"Links", meta.Links})
	}

	// Print resource-specific fields using reflection
	p.addResourceFields(resource, &fields)

	// Print all fields
	for _, field := range fields {
		if !p.shouldShowField(field.value) {
			continue
		}

		fieldName := field.name + ":"
		fieldValue := p.formatValue(field.value)

		// Handle multi-line values
		lines := strings.Split(fieldValue, "\n")
		for i, line := range lines {
			if i == 0 {
				// First line with field name
				fieldLabel := color.New(p.colors.field).Sprint(fieldName)

				// Calculate max width for the value based on visual field name length
				fieldNameVisualLen := len(fieldName)            // Use raw field name length for layout calculation
				maxValueWidth := width - fieldNameVisualLen - 6 // "│ " + fieldName + ": " + value + " │"

				// Truncate line before applying colors
				if len(line) > maxValueWidth {
					line = line[:maxValueWidth-3] + "..."
				}

				// Apply colors after truncation
				var coloredLine string
				if field.name == "Status" && line != "" {
					coloredLine = color.New(p.getStatusColor(line)).Sprint(line)
				} else if line != "" {
					coloredLine = color.New(p.colors.value).Sprint(line)
				} else {
					coloredLine = line
				}

				// Calculate padding based on visual length
				// Format: "│ fieldName value%s │" where %s is padding
				// Use visual length to account for ANSI color codes
				fieldLabelVisualLen := visualLength(fieldLabel)
				coloredLineVisualLen := visualLength(coloredLine)
				totalVisualLength := 2 + fieldLabelVisualLen + 1 + coloredLineVisualLen + 2 // "│ " + field + " " + value + " │"
				padding := width - totalVisualLength
				if padding < 0 {
					padding = 0
				}
				fmt.Fprintf(p.options.Writer, "│ %s %s%s │\n",
					fieldLabel, coloredLine, strings.Repeat(" ", padding))
			} else {
				// Continuation lines
				indentSpaces := len(fieldName) + 1
				maxValueWidth := width - indentSpaces - 6 // "│ " + indent + " " + value + " │"
				if len(line) > maxValueWidth {
					line = line[:maxValueWidth-3] + "..."
				}

				coloredLine := color.New(p.colors.value).Sprint(line)
				// For continuation lines, the format is "│ [spaces] value%s │"
				coloredLineVisualLen := visualLength(coloredLine)
				totalVisualLength := 2 + indentSpaces + 1 + coloredLineVisualLen + 2 // "│ " + indent + " " + value + " │"
				padding := width - totalVisualLength
				if padding < 0 {
					padding = 0
				}
				fmt.Fprintf(p.options.Writer, "│ %s %s%s │\n",
					strings.Repeat(" ", indentSpaces), coloredLine, strings.Repeat(" ", padding))
			}
		}
	}

	// Print bottom border
	fmt.Fprintf(p.options.Writer, "└%s┘\n", strings.Repeat("─", width-2))

	return nil
}

// addResourceFields adds resource-specific fields using reflection
func (p *ResourcePrinter) addResourceFields(resource any, fields *[]struct {
	name  string
	value interface{}
}) {
	v := reflect.ValueOf(resource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Skip embedded ResourceBase and handle embedded structs
		if field.Type.String() == "types.ResourceBase" {
			continue
		}

		// Handle embedded structs by expanding their fields
		if field.Anonymous && fieldValue.Kind() == reflect.Struct {
			p.addEmbeddedFields(fieldValue, fields)
			continue
		}

		// Skip Meta field as we handle it separately
		if field.Name == "Meta" {
			continue
		}

		// Get the field name from JSON tag or use field name
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			if tagName := strings.Split(jsonTag, ",")[0]; tagName != "" && tagName != "-" {
				fieldName = tagName
			}
		}

		// Add the field if it should be shown
		value := fieldValue.Interface()
		if p.shouldShowField(value) || p.options.ShowEmpty {
			*fields = append(*fields, struct {
				name  string
				value interface{}
			}{fieldName, value})
		}
	}
}

// addEmbeddedFields recursively adds fields from embedded structs
func (p *ResourcePrinter) addEmbeddedFields(embeddedStruct reflect.Value, fields *[]struct {
	name  string
	value interface{}
}) {
	if embeddedStruct.Kind() == reflect.Ptr {
		embeddedStruct = embeddedStruct.Elem()
	}

	if embeddedStruct.Kind() != reflect.Struct {
		return
	}

	t := embeddedStruct.Type()
	for i := 0; i < embeddedStruct.NumField(); i++ {
		field := t.Field(i)
		fieldValue := embeddedStruct.Field(i)

		if !fieldValue.CanInterface() {
			continue
		}

		// Skip ResourceBase even in embedded structs
		if field.Type.String() == "types.ResourceBase" {
			continue
		}

		// Skip Meta field
		if field.Name == "Meta" {
			continue
		}

		// Handle nested embedded structs
		if field.Anonymous && fieldValue.Kind() == reflect.Struct {
			p.addEmbeddedFields(fieldValue, fields)
			continue
		}

		// Get the field name from JSON tag or use field name
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			if tagName := strings.Split(jsonTag, ",")[0]; tagName != "" && tagName != "-" {
				fieldName = tagName
			}
		}

		// Add the field if it should be shown
		value := fieldValue.Interface()
		if p.shouldShowField(value) || p.options.ShowEmpty {
			*fields = append(*fields, struct {
				name  string
				value interface{}
			}{fieldName, value})
		}
	}
}

// printJSON prints a resource in JSON format with basic syntax highlighting
func (p *ResourcePrinter) printJSON(resource any) error {
	// Convert resource to JSON
	data, err := json.MarshalIndent(resource, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal resource to JSON: %w", err)
	}

	if *p.options.ColorEnabled {
		// Basic syntax highlighting for JSON
		jsonStr := string(data)
		jsonStr = p.highlightJSON(jsonStr)
		fmt.Fprint(p.options.Writer, jsonStr)
	} else {
		fmt.Fprint(p.options.Writer, string(data))
	}
	fmt.Fprintln(p.options.Writer)

	return nil
}

// highlightJSON provides basic syntax highlighting for JSON output
func (p *ResourcePrinter) highlightJSON(jsonStr string) string {
	lines := strings.Split(jsonStr, "\n")
	for i, line := range lines {
		// Highlight field names (strings before :)
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				fieldPart := color.New(p.colors.field).Sprint(parts[0])
				valuePart := parts[1]

				// Highlight string values
				if strings.Contains(valuePart, "\"") && !strings.HasSuffix(strings.TrimSpace(valuePart), "{") {
					valuePart = color.New(p.colors.value).Sprint(valuePart)
				}

				lines[i] = fieldPart + ":" + valuePart
			}
		}
	}
	return strings.Join(lines, "\n")
}

// printTree prints a resource in tree format with Unicode characters and emojis
func (p *ResourcePrinter) printTree(resource any) error {
	meta, err := types.GetMeta(resource)
	if err != nil {
		return fmt.Errorf("resource does not have ResourceBase: %w", err)
	}

	// Get emoji for resource type
	emoji := p.getResourceEmoji(meta.Type)

	// Print root resource
	statusColor := color.New(p.getStatusColor(meta.Status))
	fmt.Fprintf(p.options.Writer, "%s %s %s\n",
		emoji,
		color.New(p.colors.header).Sprint(meta.ID),
		statusColor.Sprintf("(%s)", meta.Status))

	// Collect fields to display
	fields := []struct {
		name  string
		value interface{}
		emoji string
	}{
		{"Type", meta.Type, "📋"},
		{"File", fmt.Sprintf("%s:%d", meta.File, meta.Line), "📁"},
		{"Module", meta.Module, "📦"},
	}

	// Add dependencies if they exist
	deps, err := types.GetDependencies(resource)
	if err != nil {
		deps = []string{} // Use empty slice if unable to get dependencies
	}
	if len(deps) > 0 {
		fields = append(fields, struct {
			name  string
			value interface{}
			emoji string
		}{"Dependencies", deps, "🔗"})
	}

	// Add resource-specific fields
	p.addTreeFields(resource, &fields)

	// Print fields
	for i, field := range fields {
		if !p.shouldShowField(field.value) {
			continue
		}

		isLast := i == len(fields)-1
		prefix := "├── "
		if isLast {
			prefix = "└── "
		}

		fieldValue := p.formatValue(field.value)
		if fieldValue != "" {
			fmt.Fprintf(p.options.Writer, "%s%s %s: %s\n",
				prefix,
				field.emoji,
				color.New(p.colors.field).Sprint(field.name),
				color.New(p.colors.value).Sprint(fieldValue))
		}
	}

	return nil
}

// addTreeFields adds resource-specific fields for tree format
func (p *ResourcePrinter) addTreeFields(resource any, fields *[]struct {
	name  string
	value interface{}
	emoji string
}) {
	v := reflect.ValueOf(resource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Skip embedded ResourceBase and Meta
		if field.Type.String() == "types.ResourceBase" || field.Name == "Meta" {
			continue
		}

		// Handle embedded structs by expanding their fields
		if field.Anonymous && fieldValue.Kind() == reflect.Struct {
			p.addEmbeddedTreeFields(fieldValue, fields)
			continue
		}

		// Get the field name from JSON tag or use field name
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			if tagName := strings.Split(jsonTag, ",")[0]; tagName != "" && tagName != "-" {
				fieldName = tagName
			}
		}

		// Get emoji for field type
		emoji := p.getFieldEmoji(fieldName, fieldValue.Interface())

		// Add the field if it should be shown
		value := fieldValue.Interface()
		if p.shouldShowField(value) {
			*fields = append(*fields, struct {
				name  string
				value interface{}
				emoji string
			}{fieldName, value, emoji})
		}
	}
}

// addEmbeddedTreeFields recursively adds fields from embedded structs for tree format
func (p *ResourcePrinter) addEmbeddedTreeFields(embeddedStruct reflect.Value, fields *[]struct {
	name  string
	value interface{}
	emoji string
}) {
	if embeddedStruct.Kind() == reflect.Ptr {
		embeddedStruct = embeddedStruct.Elem()
	}

	if embeddedStruct.Kind() != reflect.Struct {
		return
	}

	t := embeddedStruct.Type()
	for i := 0; i < embeddedStruct.NumField(); i++ {
		field := t.Field(i)
		fieldValue := embeddedStruct.Field(i)

		if !fieldValue.CanInterface() {
			continue
		}

		// Skip ResourceBase even in embedded structs
		if field.Type.String() == "types.ResourceBase" {
			continue
		}

		// Skip Meta field
		if field.Name == "Meta" {
			continue
		}

		// Handle nested embedded structs
		if field.Anonymous && fieldValue.Kind() == reflect.Struct {
			p.addEmbeddedTreeFields(fieldValue, fields)
			continue
		}

		// Get the field name from JSON tag or use field name
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			if tagName := strings.Split(jsonTag, ",")[0]; tagName != "" && tagName != "-" {
				fieldName = tagName
			}
		}

		// Get emoji for field type
		emoji := p.getFieldEmoji(fieldName, fieldValue.Interface())

		// Add the field if it should be shown
		value := fieldValue.Interface()
		if p.shouldShowField(value) {
			*fields = append(*fields, struct {
				name  string
				value interface{}
				emoji string
			}{fieldName, value, emoji})
		}
	}
}

// getResourceEmoji returns an emoji for a resource type
func (p *ResourcePrinter) getResourceEmoji(resourceType string) string {
	switch resourceType {
	case "container":
		return "🐳"
	case "network":
		return "🌐"
	case "volume":
		return "💾"
	case "variable":
		return "🔧"
	case "output":
		return "📤"
	case "module":
		return "📦"
	case "template":
		return "📄"
	default:
		return "📦"
	}
}

// getFieldEmoji returns an emoji for a field based on its name and type
func (p *ResourcePrinter) getFieldEmoji(fieldName string, value interface{}) string {
	// Check field name patterns
	switch {
	case strings.Contains(strings.ToLower(fieldName), "command"):
		return "⚡"
	case strings.Contains(strings.ToLower(fieldName), "port"):
		return "🔌"
	case strings.Contains(strings.ToLower(fieldName), "network"):
		return "🌐"
	case strings.Contains(strings.ToLower(fieldName), "volume"):
		return "📂"
	case strings.Contains(strings.ToLower(fieldName), "env"):
		return "🌍"
	case strings.Contains(strings.ToLower(fieldName), "image"):
		return "🖼️"
	case strings.Contains(strings.ToLower(fieldName), "dns"):
		return "🌐"
	case strings.Contains(strings.ToLower(fieldName), "user"):
		return "👤"
	case strings.Contains(strings.ToLower(fieldName), "resource"):
		return "💪"
	default:
		// Check value type
		v := reflect.ValueOf(value)
		if v.Kind() == reflect.Ptr && !v.IsNil() {
			v = v.Elem()
		}

		switch v.Kind() {
		case reflect.Slice, reflect.Array:
			return "📋"
		case reflect.Map:
			return "🗂️"
		case reflect.Bool:
			return "☑️"
		case reflect.String:
			return "📝"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
			return "🔢"
		default:
			return "📄"
		}
	}
}

// printCard prints a resource in card format with rounded borders
func (p *ResourcePrinter) printCard(resource any) error {
	meta, err := types.GetMeta(resource)
	if err != nil {
		return fmt.Errorf("resource does not have ResourceBase: %w", err)
	}

	// Calculate the width for the card
	width := p.options.MaxWidth
	if width < 50 {
		width = 50
	}

	// Get emoji and format header
	emoji := p.getResourceEmoji(meta.Type)
	resourceType := strings.Title(meta.Type)
	header := fmt.Sprintf("%s %s: %s", emoji, resourceType, meta.Name)
	subHeader := meta.ID

	// Print top border
	fmt.Fprintf(p.options.Writer, "╭%s╮\n", strings.Repeat("─", width-2))

	// Print header
	headerPadding := width - len(header) - 2
	if headerPadding < 0 {
		header = header[:width-5] + "..."
		headerPadding = 0
	}
	fmt.Fprintf(p.options.Writer, "│ %s%s │\n",
		color.New(p.colors.header).Sprint(header),
		strings.Repeat(" ", headerPadding))

	// Print sub-header
	subHeaderPadding := width - len(subHeader) - 4
	if subHeaderPadding < 0 {
		subHeader = subHeader[:width-7] + "..."
		subHeaderPadding = 0
	}
	fmt.Fprintf(p.options.Writer, "│ %s%s │\n",
		color.New(color.FgHiBlack).Sprint("📍 "+subHeader),
		strings.Repeat(" ", subHeaderPadding))

	// Print separator
	fmt.Fprintf(p.options.Writer, "├%s┤\n", strings.Repeat("─", width-2))

	// Print status with color
	statusText := fmt.Sprintf("Status: %s", meta.Status)
	statusEmoji := ""
	switch meta.Status {
	case "created":
		statusEmoji = "✅"
	case "pending":
		statusEmoji = "🟡"
	case "failed":
		statusEmoji = "❌"
	default:
		statusEmoji = "⚪"
	}

	statusLine := fmt.Sprintf("%s %s", statusEmoji, statusText)
	statusPadding := width - len(statusLine) - 2
	if statusPadding < 0 {
		statusPadding = 0
	}

	fmt.Fprintf(p.options.Writer, "│ %s %s%s │\n",
		statusEmoji,
		color.New(p.getStatusColor(meta.Status)).Sprint(statusText),
		strings.Repeat(" ", statusPadding))

	// Print file location
	fileText := fmt.Sprintf("File: %s:%d", meta.File, meta.Line)
	filePadding := width - len(fileText) - 2
	if filePadding < 0 {
		fileText = fileText[:width-5] + "..."
		filePadding = 0
	}
	fmt.Fprintf(p.options.Writer, "│ %s%s │\n",
		color.New(p.colors.value).Sprint(fileText),
		strings.Repeat(" ", filePadding))

	// Add empty line
	fmt.Fprintf(p.options.Writer, "│%s │\n", strings.Repeat(" ", width-2))

	// Print key resource fields
	p.printCardFields(resource, width)

	// Print bottom border
	fmt.Fprintf(p.options.Writer, "╰%s╯\n", strings.Repeat("─", width-2))

	return nil
}

// printCardFields prints key fields for card format
func (p *ResourcePrinter) printCardFields(resource any, width int) {
	v := reflect.ValueOf(resource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	// Print a few key fields in card format
	keyFields := []string{"command", "image", "networks", "ports", "volumes"}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if !fieldValue.CanInterface() {
			continue
		}

		if field.Type.String() == "types.ResourceBase" || field.Name == "Meta" {
			continue
		}

		// Get the field name from JSON tag
		fieldName := strings.ToLower(field.Name)
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			if tagName := strings.Split(jsonTag, ",")[0]; tagName != "" && tagName != "-" {
				fieldName = strings.ToLower(tagName)
			}
		}

		// Only show key fields in card format
		isKeyField := false
		for _, key := range keyFields {
			if strings.Contains(fieldName, key) {
				isKeyField = true
				break
			}
		}

		if !isKeyField || !p.shouldShowField(fieldValue.Interface()) {
			continue
		}

		// Format and print the field
		displayName := strings.Title(strings.ReplaceAll(fieldName, "_", " "))
		fieldValueStr := p.formatValue(fieldValue.Interface())

		if len(fieldValueStr) > 0 {
			line := fmt.Sprintf("%s: %s", displayName, fieldValueStr)
			if len(line) > width-4 {
				line = line[:width-7] + "..."
			}
			padding := width - len(line) - 2
			if padding < 0 {
				padding = 0
			}
			fmt.Fprintf(p.options.Writer, "│ %s%s │\n",
				color.New(p.colors.value).Sprint(line),
				strings.Repeat(" ", padding))
		}
	}
}
