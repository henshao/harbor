// Copyright Project Harbor Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// TagConversion represents a mapping from Beego tag to Bun tag
type TagConversion struct {
	BeegoPattern string
	BunTag       string
	Description  string
}

// Tag conversion mappings
var tagConversions = []TagConversion{
	{`pk;auto;column\(([^)]+)\)`, `",pk,autoincrement"`, "Primary key with auto increment"},
	{`pk;auto`, `",pk,autoincrement"`, "Primary key with auto increment (no column name)"},
	{`pk;column\(([^)]+)\)`, `",pk"`, "Primary key"},
	{`pk`, `",pk"`, "Primary key (no column name)"},
	{`column\(([^)]+)\);auto_now_add`, `",nullzero,notnull,default:current_timestamp"`, "Creation timestamp"},
	{`column\(([^)]+)\);auto_now`, `",nullzero,notnull,default:current_timestamp"`, "Update timestamp"},
	{`auto_now_add`, `",nullzero,notnull,default:current_timestamp"`, "Creation timestamp (no column name)"},
	{`auto_now`, `",nullzero,notnull,default:current_timestamp"`, "Update timestamp (no column name)"},
	{`column\(([^)]+)\);size\(([^)]+)\)`, `"$1,type:varchar($2)"`, "Column with size"},
	{`column\(([^)]+)\);unique`, `"$1,unique,notnull"`, "Unique column"},
	{`column\(([^)]+)\);null`, `"$1,nullzero"`, "Nullable column"},
	{`column\(([^)]+)\)`, `"$1"`, "Regular column"},
	{`size\(([^)]+)\)`, `",type:varchar($1)"`, "Size specification"},
	{`unique`, `",unique"`, "Unique constraint"},
	{`null`, `",nullzero"`, "Nullable field"},
	{`-`, `"-"`, "Ignored field"},
}

// ModelInfo holds information about a parsed model
type ModelInfo struct {
	Name      string
	TableName string
	Fields    []FieldInfo
	Methods   []string
}

// FieldInfo holds information about a model field
type FieldInfo struct {
	Name       string
	Type       string
	BeegoTag   string
	BunTag     string
	JSONTag    string
	OtherTags  map[string]string
	Comment    string
}

// parseFile parses a Go file and extracts model information
func parseFile(filename string) ([]ModelInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var models []ModelInfo

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			if structType, ok := x.Type.(*ast.StructType); ok {
				model := parseStruct(x.Name.Name, structType)
				if model != nil {
					models = append(models, *model)
				}
			}
		}
		return true
	})

	return models, nil
}

// parseStruct parses a struct and extracts field information
func parseStruct(name string, structType *ast.StructType) *ModelInfo {
	model := &ModelInfo{
		Name:   name,
		Fields: make([]FieldInfo, 0),
	}

	hasORMTag := false

	for _, field := range structType.Fields.List {
		if field.Tag != nil {
			tagValue := field.Tag.Value
			if strings.Contains(tagValue, `orm:"`) {
				hasORMTag = true
			}

			for _, fieldName := range field.Names {
				fieldInfo := parseField(fieldName.Name, field)
				model.Fields = append(model.Fields, fieldInfo)
			}
		}
	}

	// Only return models that have ORM tags
	if !hasORMTag {
		return nil
	}

	return model
}

// parseField parses a single field and converts tags
func parseField(name string, field *ast.Field) FieldInfo {
	fieldInfo := FieldInfo{
		Name:      name,
		Type:      typeToString(field.Type),
		OtherTags: make(map[string]string),
	}

	if field.Tag != nil {
		tags := parseStructTag(field.Tag.Value)
		
		if ormTag, exists := tags["orm"]; exists {
			fieldInfo.BeegoTag = ormTag
			fieldInfo.BunTag = convertBeegoTagToBun(ormTag)
		}

		if jsonTag, exists := tags["json"]; exists {
			fieldInfo.JSONTag = jsonTag
		}

		for key, value := range tags {
			if key != "orm" && key != "json" {
				fieldInfo.OtherTags[key] = value
			}
		}
	}

	return fieldInfo
}

// convertBeegoTagToBun converts a Beego ORM tag to Bun ORM tag
func convertBeegoTagToBun(beegoTag string) string {
	for _, conversion := range tagConversions {
		re := regexp.MustCompile(conversion.BeegoPattern)
		if re.MatchString(beegoTag) {
			return re.ReplaceAllString(beegoTag, conversion.BunTag)
		}
	}
	
	// If no specific conversion found, return as-is with quotes
	if beegoTag == "-" {
		return `"-"`
	}
	
	return fmt.Sprintf(`"%s"`, beegoTag)
}

// parseStructTag parses struct tag string into map
func parseStructTag(tag string) map[string]string {
	tags := make(map[string]string)
	
	// Remove surrounding backticks
	tag = strings.Trim(tag, "`")
	
	// Regular expression to match tag patterns
	re := regexp.MustCompile(`(\w+):"([^"]*)"`)
	matches := re.FindAllStringSubmatch(tag, -1)
	
	for _, match := range matches {
		if len(match) == 3 {
			tags[match[1]] = match[2]
		}
	}
	
	return tags
}

// typeToString converts ast.Expr to string representation
func typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return typeToString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + typeToString(t.Key) + "]" + typeToString(t.Value)
	case *ast.StarExpr:
		return "*" + typeToString(t.X)
	default:
		return "interface{}"
	}
}

// generateBunModel generates Bun model code
func generateBunModel(model ModelInfo) string {
	var builder strings.Builder
	
	builder.WriteString(fmt.Sprintf("// Bun%s is the Bun ORM version of %s\n", model.Name, model.Name))
	builder.WriteString("// Migrated from Beego ORM to Bun ORM\n")
	builder.WriteString(fmt.Sprintf("type Bun%s struct {\n", model.Name))
	
	// Add BaseModel if we have a table
	if model.TableName != "" {
		builder.WriteString(fmt.Sprintf("\tbun.BaseModel `bun:\"table:%s\"`\n\n", model.TableName))
	} else {
		builder.WriteString("\tbun.BaseModel\n\n")
	}
	
	for _, field := range model.Fields {
		if field.BeegoTag != "" {
			builder.WriteString(fmt.Sprintf("\t// Beego: orm:\"%s\" -> Bun: bun:%s\n", field.BeegoTag, field.BunTag))
		}
		
		// Build the field line
		fieldLine := fmt.Sprintf("\t%s %s", field.Name, field.Type)
		
		// Add tags
		var tags []string
		if field.BunTag != "" {
			tags = append(tags, fmt.Sprintf("bun:%s", field.BunTag))
		}
		if field.JSONTag != "" {
			tags = append(tags, fmt.Sprintf("json:\"%s\"", field.JSONTag))
		}
		for key, value := range field.OtherTags {
			tags = append(tags, fmt.Sprintf("%s:\"%s\"", key, value))
		}
		
		if len(tags) > 0 {
			fieldLine += " `" + strings.Join(tags, " ") + "`"
		}
		
		builder.WriteString(fieldLine + "\n")
	}
	
	builder.WriteString("}\n")
	
	return builder.String()
}

// main function
func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run beego_to_bun.go <input_file>")
	}

	filename := os.Args[1]
	
	models, err := parseFile(filename)
	if err != nil {
		log.Fatalf("Error parsing file: %v", err)
	}

	if len(models) == 0 {
		fmt.Println("No models with ORM tags found in the file.")
		return
	}

	// Generate output filename
	dir := filepath.Dir(filename)
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	outputFile := filepath.Join(dir, fmt.Sprintf("bun_%s%s", name, ext))

	// Generate file content
	var output strings.Builder
	output.WriteString("// Auto-generated Bun ORM models from Beego ORM\n")
	output.WriteString("// Generated by beego_to_bun migration tool\n\n")
	output.WriteString("package models\n\n")
	output.WriteString("import (\n")
	output.WriteString("\t\"time\"\n")
	output.WriteString("\t\"github.com/uptrace/bun\"\n")
	output.WriteString(")\n\n")

	for _, model := range models {
		output.WriteString(generateBunModel(model))
		output.WriteString("\n")
	}

	// Write to output file
	err = os.WriteFile(outputFile, []byte(output.String()), 0644)
	if err != nil {
		log.Fatalf("Error writing output file: %v", err)
	}

	fmt.Printf("Successfully converted %d models from %s to %s\n", len(models), filename, outputFile)
	
	// Print summary
	fmt.Println("\nConverted models:")
	for _, model := range models {
		fmt.Printf("- %s -> Bun%s (%d fields)\n", model.Name, model.Name, len(model.Fields))
	}
	
	fmt.Println("\nNext steps:")
	fmt.Println("1. Review the generated file and adjust as needed")
	fmt.Println("2. Add necessary imports")
	fmt.Println("3. Update table names if needed")
	fmt.Println("4. Test the generated models")
	fmt.Println("5. Update DAO layer to use Bun queries")
}