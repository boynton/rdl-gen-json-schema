// Copyright 2018 Lee Boynton
// Licensed under the terms of the Apache version 2.0 license. See LICENSE file for terms.
package main

//
// export RDL types to JSON Schema
//

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ardielle/ardielle-go/rdl"
)

func main() {
	outdir := flag.String("o", "", "Output directory")
	flag.String("s", "", "RDL source file") //ignored, but not an error if provided
	basePath := flag.String("b", "", "Base path")
	flag.Parse()
	data, err := ioutil.ReadAll(os.Stdin)
	if err == nil {
		var schema rdl.Schema
		err = json.Unmarshal(data, &schema)
		if err == nil {
			ExportToJSONSchema(&schema, *outdir, *basePath)
			os.Exit(0)
		}
	}
	fmt.Fprintf(os.Stderr, "*** %v\n", err)
	os.Exit(1)
}

func outputWriter(outdir string, name string, ext string) (*bufio.Writer, *os.File, string, error) {
	sname := "anonymous"
	if strings.HasSuffix(outdir, ext) {
		name = filepath.Base(outdir)
		sname = name[:len(name)-len(ext)]
		outdir = filepath.Dir(outdir)
	}
	if name != "" {
		sname = name
	}
	if outdir == "" {
		return bufio.NewWriter(os.Stdout), nil, sname, nil
	}
	outfile := sname
	if !strings.HasSuffix(outfile, ext) {
		outfile += ext
	}
	path := filepath.Join(outdir, outfile)
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, "", err
	}
	writer := bufio.NewWriter(f)
	return writer, f, sname, nil
}

// ExportToJSONSchema exports the types in the RDL schema to a single Json Schema file of definitions
func ExportToJSONSchema(schema *rdl.Schema, outdir string, basePath string) error {
	sname := string(schema.Name)
	js, err := jsGenerate(schema, basePath)
	if err != nil {
		return err
	}
	j, err := json.MarshalIndent(js, "", "    ")
	if err != nil {
		return err
	}
	out, file, _, err := outputWriter(outdir, sname, ".json")
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s\n", string(j))
	out.Flush()
	if file != nil {
		file.Close()
	}
	return err
}

//always generates schemas of the form {"definitions": { ... }}, unless no types are defined at all, then just {}
func jsGenerate(schema *rdl.Schema, basePath string) (map[string]interface{}, error) {
	reg := rdl.NewTypeRegistry(schema)
	js := make(map[string]interface{})
	js["$schema"] = "http://json-schema.org/draft-04/schema#"
	if len(schema.Types) > 0 {
		defs := make(map[string]map[string]interface{})
		js["definitions"] = defs
		for _, t := range schema.Types {
			ref := jsTypeDef(reg, t)
			if ref != nil {
				tName, _, _ := rdl.TypeInfo(t)
				defs[string(tName)] = ref
			}
		}
	}
	return js, nil
}

func jsTypeRef(reg rdl.TypeRegistry, itemTypeName rdl.TypeRef) (string, string, interface{}) {
	itype := string(itemTypeName)
	switch reg.FindBaseType(itemTypeName) {
	case rdl.BaseTypeInt8:
		return "string", "byte", nil //?
	case rdl.BaseTypeInt16, rdl.BaseTypeInt32, rdl.BaseTypeInt64:
		return "integer", strings.ToLower(itype), nil
	case rdl.BaseTypeFloat32:
		return "number", "float", nil
	case rdl.BaseTypeFloat64:
		return "number", "double", nil
	case rdl.BaseTypeString:
		return "string", "", nil
	case rdl.BaseTypeTimestamp:
		return "string", "date-time", nil
	case rdl.BaseTypeUUID, rdl.BaseTypeSymbol:
		return "string", strings.ToLower(itype), nil
	default:
		s := make(map[string]interface{})
		s["$ref"] = "#/definitions/" + itype
		return "", "", s
	}
}

func jsTypeDef(reg rdl.TypeRegistry, t *rdl.Type) map[string]interface{} {
	st := make(map[string]interface{})
	bt := reg.BaseType(t)
	switch t.Variant {
	case rdl.TypeVariantStructTypeDef:
		typedef := t.StructTypeDef
		if typedef.Comment != "" {
			st["description"] = typedef.Comment
		}
		props := make(map[string]interface{})
		var required []string
		if len(typedef.Fields) > 0 {
			for _, f := range typedef.Fields {
				if !f.Optional {
					required = append(required, string(f.Name))
				}
				ft := reg.FindType(f.Type)
				fbt := reg.BaseType(ft)
				prop := make(map[string]interface{})
				if f.Comment != "" {
					prop["description"] = f.Comment
				}
				switch fbt {
				case rdl.BaseTypeArray:
					prop["type"] = "array"
					if ft.Variant == rdl.TypeVariantArrayTypeDef && f.Items == "" {
						f.Items = ft.ArrayTypeDef.Items
					}
					if f.Items != "" {
						fitems := string(f.Items)
						items := make(map[string]interface{})
						switch fitems {
						case "String":
							items["type"] = strings.ToLower(fitems)
						case "Int32", "Int64", "Int16":
							items["type"] = "integer"
							//not supported by all validators: items["format"] = strings.ToLower(fitems)
						default:
							items["$ref"] = "#/definitions/" + fitems
						}
						prop["items"] = items
					}
				case rdl.BaseTypeString:
					if ft.Variant != rdl.TypeVariantBaseType {
						name, _, _ := rdl.TypeInfo(ft)
						prop["$ref"] = "#/definitions/" + name
					} else {
						prop["type"] = "string"
					}
				case rdl.BaseTypeInt32, rdl.BaseTypeInt64, rdl.BaseTypeInt16:
					prop["type"] = "integer"
					//not always supported prop["format"] = strings.ToLower(fbt.String())
				case rdl.BaseTypeStruct:
					prop["$ref"] = "#/definitions/" + string(f.Type)
				case rdl.BaseTypeMap:
					prop["type"] = "object"
					if f.Items != "" {
						fitems := string(f.Items)
						items := make(map[string]interface{})
						switch f.Items {
						case "String":
							items["type"] = strings.ToLower(fitems)
						case "Int32", "Int64", "Int16":
							items["type"] = "integer"
							items["format"] = strings.ToLower(fitems)
						default:
							items["$ref"] = "#/definitions/" + fitems
						}
						prop["additionalProperties"] = items
					}
				case rdl.BaseTypeEnum:
					prop["$ref"] = "#/definitions/" + string(f.Type)
				default:
					prop["type"] = "_" + string(f.Type) + "_" //!
				}
				props[string(f.Name)] = prop
			}
		}
		st["properties"] = props
		if len(required) > 0 {
			st["required"] = required
		}
	case rdl.TypeVariantMapTypeDef:
		typedef := t.MapTypeDef
		st["type"] = "object"
		if typedef.Items != "Any" {
			items := make(map[string]interface{})
			switch reg.FindBaseType(typedef.Items) {
			case rdl.BaseTypeString:
				items["type"] = strings.ToLower(string(typedef.Items))
			case rdl.BaseTypeInt32, rdl.BaseTypeInt64, rdl.BaseTypeInt16:
				items["type"] = "integer"
				items["format"] = strings.ToLower(string(typedef.Items))
			default:
				items["$ref"] = "#/definitions/" + string(typedef.Items)
			}
			st["additionalProperties"] = items
		}
	case rdl.TypeVariantArrayTypeDef:
		typedef := t.ArrayTypeDef
		st["type"] = bt.String()
		if typedef.Items != "Any" {
			items := make(map[string]interface{})
			switch reg.FindBaseType(typedef.Items) {
			case rdl.BaseTypeString:
				items["type"] = strings.ToLower(string(typedef.Items))
			case rdl.BaseTypeInt32, rdl.BaseTypeInt64, rdl.BaseTypeInt16:
				items["type"] = "integer"
				items["format"] = strings.ToLower(string(typedef.Items))
			default:
				items["$ref"] = "#/definitions/" + string(typedef.Items)
			}
			st["items"] = items
		}
	case rdl.TypeVariantEnumTypeDef:
		typedef := t.EnumTypeDef
		var tmp []string
		for _, el := range typedef.Elements {
			tmp = append(tmp, string(el.Symbol))
		}
		st["enum"] = tmp
	case rdl.TypeVariantUnionTypeDef:
		typedef := t.UnionTypeDef
		fmt.Println("[" + typedef.Name + ": Unions not supported]")
	default:
		switch bt {
		case rdl.BaseTypeString:
			if t.StringTypeDef != nil {
				typedef := t.StringTypeDef
				st["type"] = "string"
				if typedef.MaxSize != nil {
					st["maxLength"] = *typedef.MaxSize
				}
				if typedef.MinSize != nil {
					st["minLength"] = *typedef.MinSize
				}
				if typedef.Pattern != "" {
					st["pattern"] = typedef.Pattern
				}
			} else {
				return nil
			}
		case rdl.BaseTypeInt16, rdl.BaseTypeInt32, rdl.BaseTypeInt64, rdl.BaseTypeFloat32, rdl.BaseTypeFloat64:
			return nil
		case rdl.BaseTypeStruct:
			st["type"] = "object"
		default:
			panic(fmt.Sprintf("whoops: %v", t))
		}
	}
	return st
}
