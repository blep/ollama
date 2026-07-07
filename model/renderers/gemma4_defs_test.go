package renderers

import (
    "fmt"
    "strings"
    "testing"

    "github.com/ollama/ollama/api"
)

func TestDefsRefResolution(t *testing.T) {
    r := &Gemma4Renderer{}

    // Build a schema that uses $defs/$ref, exactly like pydantic_function_tool generates
    props := api.NewToolPropertiesMap()

    // $ref items — the items field will contain {"$ref": "#/$defs/Idea"}
    itemsMap := map[string]any{
        "$ref": "#/$defs/Idea",
    }

    props.Set("ideas", api.ToolProperty{
        Type: api.PropertyType{"array"},
        Items: itemsMap,
        Description: "List of ideas",
    })

    tool := api.Tool{
        Function: api.ToolFunction{
            Name:        "create_ideas",
            Description: "Create ideas",
            Parameters: api.ToolFunctionParameters{
                Type:       "object",
                Properties: props,
                Required:   []string{"ideas"},
                // The $defs section as it arrives from JSON unmarshalling
                Defs: map[string]any{
                    "Idea": map[string]any{
                        "type": "object",
                        "properties": map[string]any{
                            "content": map[string]any{
                                "type": "string",
                                "description": "The content of the idea",
                            },
                        },
                        "required": []any{"content"},
                    },
                },
            },
        },
    }

    result := r.renderToolDeclaration(tool)
    fmt.Println("Rendered tool declaration:")
    fmt.Println(result)

    // Verify the $ref was resolved:
    // 1. Should NOT contain "$ref" or "#/$defs/Idea"
    if strings.Contains(result, "$ref") || strings.Contains(result, "#/$defs/Idea") {
        t.Error("$ref was not resolved — still contains raw ref")
    }
    // 2. Should contain the resolved property "content" from the Idea def
    if !strings.Contains(result, "content:{") {
        t.Error("Resolved Idea schema missing 'content' property")
    }
    // 3. Should contain the nested properties block
    if !strings.Contains(result, "properties:{content:") {
        t.Error("Nested properties from Idea def not rendered correctly")
    }
}
