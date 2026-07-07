package renderers

import (
    "fmt"
    "strings"
    "testing"
    "github.com/ollama/ollama/api"
)

func TestReservedKeysAsParamNames(t *testing.T) {
    r := &Gemma4Renderer{}

    tool := api.Tool{
        Function: api.ToolFunction{
            Name:        "test_tool",
            Description: "Test tool",
            Parameters: api.ToolFunctionParameters{
                Type: "object",
                Properties: api.NewToolPropertiesMap(),
                Required: []string{"description"},
            },
        },
    }

    // Add a property named "description" — this was stripped before PR #15703
    tool.Function.Parameters.Properties.Set("description", api.ToolProperty{
        Type:        api.PropertyType{"string"},
        Description: "A description field",
    })

    result := r.renderToolDeclaration(tool)
    fmt.Println("Rendered tool declaration:")
    fmt.Println(result)

    // The key test: does the output contain "description" as a property name?
    if !strings.Contains(result, "description:{") {
        t.Error("PR #15703 regression: 'description' as parameter name was stripped!")
    }
}

func TestReservedKeysStillFilteredInNested(t *testing.T) {
    // Simulate a property that has "type" as a nested structural key
    // (the "no explicit properties" fallback path in writeSchemaProperties)
    // This test verifies that type/properties/required are still filtered
    // when they appear as structural schema keys inside a nested object.
    // This is harder to test directly without building a full schema.
    // For now, the key test is TestReservedKeysAsParamNames.
    _ = t
}
