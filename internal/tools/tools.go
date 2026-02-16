package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mistakeknot/clodex/internal/classify"
	"github.com/mistakeknot/clodex/internal/extract"
)

// RegisterAll registers all clodex MCP tools.
func RegisterAll(s *server.MCPServer, dispatchPath string) {
	s.AddTools(
		extractSectionsTool(),
		classifySectionsTool(dispatchPath),
	)
}

type extractSectionResult struct {
	SectionID     int    `json:"section_id"`
	Heading       string `json:"heading"`
	LineCount     int    `json:"line_count"`
	FirstSentence string `json:"first_sentence"`
}

func extractSectionsTool() server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("extract_sections",
			mcp.WithDescription("Split markdown by ## headings while honoring fenced code blocks."),
			mcp.WithString("file_path",
				mcp.Description("Absolute or workspace-relative markdown file path"),
				mcp.Required(),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			_ = ctx
			filePath, errText := requiredString(req.GetArguments(), "file_path")
			if errText != "" {
				return mcp.NewToolResultError(errText), nil
			}

			doc, err := os.ReadFile(filePath)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("read %s: %v", filePath, err)), nil
			}

			sections := extract.ExtractSections(string(doc))
			response := make([]extractSectionResult, 0, len(sections))
			for _, section := range sections {
				response = append(response, extractSectionResult{
					SectionID:     section.ID,
					Heading:       section.Heading,
					LineCount:     section.LineCount,
					FirstSentence: section.FirstSentence(),
				})
			}
			return jsonResult(response)
		},
	}
}

func classifySectionsTool(dispatchPath string) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("classify_sections",
			mcp.WithDescription("Classify markdown sections into flux-drive domains via Codex spark dispatch."),
			mcp.WithString("file_path",
				mcp.Description("Absolute or workspace-relative markdown file path"),
				mcp.Required(),
			),
			mcp.WithArray("agents",
				mcp.Description("Optional agents override. Accepts array of names or {name,description} objects."),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			filePath, errText := requiredString(args, "file_path")
			if errText != "" {
				return mcp.NewToolResultError(errText), nil
			}

			doc, err := os.ReadFile(filePath)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("read %s: %v", filePath, err)), nil
			}

			sections := extract.ExtractSections(string(doc))
			agents := parseAgentsArg(args["agents"])
			if len(agents) == 0 {
				agents = classify.DefaultAgents()
			}

			result := classify.Classify(ctx, dispatchPath, sections, agents)
			return jsonResult(result)
		},
	}
}

func parseAgentsArg(raw any) []classify.AgentDomain {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil
	}

	defaults := classify.DefaultAgents()
	defaultDescriptions := make(map[string]string, len(defaults))
	for _, agent := range defaults {
		defaultDescriptions[agent.Name] = agent.Description
	}

	result := make([]classify.AgentDomain, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		switch v := item.(type) {
		case string:
			name := strings.TrimSpace(v)
			if name == "" || seen[name] {
				continue
			}
			result = append(result, classify.AgentDomain{
				Name:        name,
				Description: defaultDescriptions[name],
			})
			seen[name] = true
		case map[string]any:
			name, _ := v["name"].(string)
			name = strings.TrimSpace(name)
			if name == "" || seen[name] {
				continue
			}
			description, _ := v["description"].(string)
			description = strings.TrimSpace(description)
			if description == "" {
				description = defaultDescriptions[name]
			}
			result = append(result, classify.AgentDomain{
				Name:        name,
				Description: description,
			})
			seen[name] = true
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func requiredString(args map[string]any, key string) (string, string) {
	value, _ := args[key].(string)
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Sprintf("%s is required", key)
	}
	return value, ""
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	encoded, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal tool response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(encoded)), nil
}
