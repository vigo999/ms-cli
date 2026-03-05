package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// SchemaType represents JSON schema types
type SchemaType string

const (
	TypeString  SchemaType = "string"
	TypeInteger SchemaType = "integer"
	TypeNumber  SchemaType = "number"
	TypeBoolean SchemaType = "boolean"
	TypeArray   SchemaType = "array"
	TypeObject  SchemaType = "object"
)

// SchemaProperty defines a single parameter property
type SchemaProperty struct {
	Type        SchemaType                 `json:"type"`
	Description string                     `json:"description"`
	Enum        []string                   `json:"enum,omitempty"`
	Items       *SchemaProperty            `json:"items,omitempty"`      // For array types
	Properties  map[string]*SchemaProperty `json:"properties,omitempty"` // For object types
	Required    bool                       `json:"-"`                    // Used during validation, not in JSON output
	MinLength   *int                       `json:"minLength,omitempty"`
	MaxLength   *int                       `json:"maxLength,omitempty"`
	Minimum     *float64                   `json:"minimum,omitempty"`
	Maximum     *float64                   `json:"maximum,omitempty"`
	Pattern     string                     `json:"pattern,omitempty"`
	Default     any                        `json:"default,omitempty"`
}

// Schema defines the parameter schema for a tool
type Schema struct {
	Type       SchemaType                 `json:"type"`
	Properties map[string]*SchemaProperty `json:"properties"`
	Required   []string                   `json:"required"`
}

// ValidationError represents a parameter validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field %q: %s", e.Field, e.Message)
}

// ValidateAgainstSchema validates raw JSON against a schema
func ValidateAgainstSchema(data json.RawMessage, schema Schema) []ValidationError {
	var params map[string]any
	if err := json.Unmarshal(data, &params); err != nil {
		return []ValidationError{{Field: "", Message: fmt.Sprintf("invalid JSON: %v", err)}}
	}

	var errors []ValidationError

	// Check required fields
	for _, required := range schema.Required {
		if _, ok := params[required]; !ok {
			errors = append(errors, ValidationError{
				Field:   required,
				Message: "required field missing",
			})
		}
	}

	// Validate each property
	for name, prop := range schema.Properties {
		if value, ok := params[name]; ok {
			if errs := validateProperty(value, prop, name); errs != nil {
				errors = append(errors, errs...)
			}
		}
	}

	// Check for unknown properties
	for name := range params {
		if _, ok := schema.Properties[name]; !ok {
			errors = append(errors, ValidationError{
				Field:   name,
				Message: "unknown property",
			})
		}
	}

	return errors
}

func validateProperty(value any, prop *SchemaProperty, path string) []ValidationError {
	var errors []ValidationError

	// Type validation
	switch prop.Type {
	case TypeString:
		str, ok := value.(string)
		if !ok {
			return []ValidationError{{Field: path, Message: fmt.Sprintf("expected string, got %T", value)}}
		}
		if errs := validateString(str, prop, path); errs != nil {
			errors = append(errors, errs...)
		}

	case TypeInteger:
		// JSON numbers are float64 by default
		num, ok := value.(float64)
		if !ok {
			return []ValidationError{{Field: path, Message: fmt.Sprintf("expected number, got %T", value)}}
		}
		if num != float64(int64(num)) {
			return []ValidationError{{Field: path, Message: "expected integer, got float"}}
		}
		if errs := validateNumber(num, prop, path); errs != nil {
			errors = append(errors, errs...)
		}

	case TypeNumber:
		num, ok := value.(float64)
		if !ok {
			return []ValidationError{{Field: path, Message: fmt.Sprintf("expected number, got %T", value)}}
		}
		if errs := validateNumber(num, prop, path); errs != nil {
			errors = append(errors, errs...)
		}

	case TypeBoolean:
		if _, ok := value.(bool); !ok {
			return []ValidationError{{Field: path, Message: fmt.Sprintf("expected boolean, got %T", value)}}
		}

	case TypeArray:
		arr, ok := value.([]any)
		if !ok {
			return []ValidationError{{Field: path, Message: fmt.Sprintf("expected array, got %T", value)}}
		}
		for i, item := range arr {
			if prop.Items != nil {
				itemPath := fmt.Sprintf("%s[%d]", path, i)
				if errs := validateProperty(item, prop.Items, itemPath); errs != nil {
					errors = append(errors, errs...)
				}
			}
		}

	case TypeObject:
		obj, ok := value.(map[string]any)
		if !ok {
			return []ValidationError{{Field: path, Message: fmt.Sprintf("expected object, got %T", value)}}
		}
		if prop.Properties != nil {
			for name, nestedProp := range prop.Properties {
				nestedPath := fmt.Sprintf("%s.%s", path, name)
				if nestedValue, ok := obj[name]; ok {
					if errs := validateProperty(nestedValue, nestedProp, nestedPath); errs != nil {
						errors = append(errors, errs...)
					}
				} else if nestedProp.Required {
					errors = append(errors, ValidationError{
						Field:   nestedPath,
						Message: "required field missing",
					})
				}
			}
		}
	}

	// Enum validation
	if len(prop.Enum) > 0 {
		strValue := fmt.Sprintf("%v", value)
		found := false
		for _, enum := range prop.Enum {
			if strValue == enum {
				found = true
				break
			}
		}
		if !found {
			errors = append(errors, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("value must be one of: %v", prop.Enum),
			})
		}
	}

	return errors
}

func validateString(value string, prop *SchemaProperty, path string) []ValidationError {
	var errors []ValidationError

	if prop.MinLength != nil && len(value) < *prop.MinLength {
		errors = append(errors, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("string too short (min %d)", *prop.MinLength),
		})
	}

	if prop.MaxLength != nil && len(value) > *prop.MaxLength {
		errors = append(errors, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("string too long (max %d)", *prop.MaxLength),
		})
	}

	if prop.Pattern != "" {
		re, err := regexp.Compile(prop.Pattern)
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("invalid pattern: %v", err),
			})
		} else if !re.MatchString(value) {
			errors = append(errors, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("value does not match pattern: %s", prop.Pattern),
			})
		}
	}

	return errors
}

func validateNumber(value float64, prop *SchemaProperty, path string) []ValidationError {
	var errors []ValidationError

	if prop.Minimum != nil && value < *prop.Minimum {
		errors = append(errors, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("value below minimum (%v)", *prop.Minimum),
		})
	}

	if prop.Maximum != nil && value > *prop.Maximum {
		errors = append(errors, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("value above maximum (%v)", *prop.Maximum),
		})
	}

	return errors
}

// SchemaBuilder provides a fluent API for building schemas
type SchemaBuilder struct {
	schema Schema
}

// NewSchema creates a new schema builder
func NewSchema() *SchemaBuilder {
	return &SchemaBuilder{
		schema: Schema{
			Type:       TypeObject,
			Properties: make(map[string]*SchemaProperty),
		},
	}
}

func (b *SchemaBuilder) String(name, description string) *SchemaBuilder {
	b.schema.Properties[name] = &SchemaProperty{
		Type:        TypeString,
		Description: description,
	}
	return b
}

func (b *SchemaBuilder) Int(name, description string) *SchemaBuilder {
	b.schema.Properties[name] = &SchemaProperty{
		Type:        TypeInteger,
		Description: description,
	}
	return b
}

func (b *SchemaBuilder) Number(name, description string) *SchemaBuilder {
	b.schema.Properties[name] = &SchemaProperty{
		Type:        TypeNumber,
		Description: description,
	}
	return b
}

func (b *SchemaBuilder) Bool(name, description string) *SchemaBuilder {
	b.schema.Properties[name] = &SchemaProperty{
		Type:        TypeBoolean,
		Description: description,
	}
	return b
}

func (b *SchemaBuilder) Array(name, description string, items *SchemaProperty) *SchemaBuilder {
	b.schema.Properties[name] = &SchemaProperty{
		Type:        TypeArray,
		Description: description,
		Items:       items,
	}
	return b
}

func (b *SchemaBuilder) Object(name, description string, properties map[string]*SchemaProperty, required []string) *SchemaBuilder {
	// Mark required fields in properties
	for _, r := range required {
		if prop, ok := properties[r]; ok {
			prop.Required = true
		}
	}

	b.schema.Properties[name] = &SchemaProperty{
		Type:        TypeObject,
		Description: description,
		Properties:  properties,
	}
	return b
}

func (b *SchemaBuilder) Required(names ...string) *SchemaBuilder {
	b.schema.Required = append(b.schema.Required, names...)
	return b
}

func (b *SchemaBuilder) Build() Schema {
	return b.schema
}

// Helper methods for constraints

func (b *SchemaBuilder) WithMinLength(name string, min int) *SchemaBuilder {
	if prop, ok := b.schema.Properties[name]; ok && prop.Type == TypeString {
		prop.MinLength = &min
	}
	return b
}

func (b *SchemaBuilder) WithMaxLength(name string, max int) *SchemaBuilder {
	if prop, ok := b.schema.Properties[name]; ok && prop.Type == TypeString {
		prop.MaxLength = &max
	}
	return b
}

func (b *SchemaBuilder) WithMinimum(name string, min float64) *SchemaBuilder {
	if prop, ok := b.schema.Properties[name]; ok && (prop.Type == TypeInteger || prop.Type == TypeNumber) {
		prop.Minimum = &min
	}
	return b
}

func (b *SchemaBuilder) WithMaximum(name string, max float64) *SchemaBuilder {
	if prop, ok := b.schema.Properties[name]; ok && (prop.Type == TypeInteger || prop.Type == TypeNumber) {
		prop.Maximum = &max
	}
	return b
}

func (b *SchemaBuilder) WithPattern(name string, pattern string) *SchemaBuilder {
	if prop, ok := b.schema.Properties[name]; ok && prop.Type == TypeString {
		prop.Pattern = pattern
	}
	return b
}

func (b *SchemaBuilder) WithEnum(name string, enum []string) *SchemaBuilder {
	if prop, ok := b.schema.Properties[name]; ok {
		prop.Enum = enum
	}
	return b
}

func (b *SchemaBuilder) WithDefault(name string, defaultValue any) *SchemaBuilder {
	if prop, ok := b.schema.Properties[name]; ok {
		prop.Default = defaultValue
	}
	return b
}
