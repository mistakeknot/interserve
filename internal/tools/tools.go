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
	"github.com/mistakeknot/clodex/internal/query"
)

// RegisterAll registers all clodex MCP tools.
func RegisterAll(s *server.MCPServer, dispatchPath string) {
	s.AddTools(
		extractSectionsTool(),
		classifySectionsTool(dispatchPath),
		codexQueryTool(dispatchPath),
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

func codexQueryTool(dispatchPath string) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("codex_query",
			mcp.WithDescription("Ask Codex to analyze file(s) and return a compact answer. Saves Claude context by delegating file reading to Codex."),
			mcp.WithString("question",
				mcp.Description("The question about the file(s). Required for answer/extract modes."),
			),
			mcp.WithArray("files",
				mcp.Description("Absolute file paths to analyze."),
				mcp.Required(),
			),
			mcp.WithString("mode",
				mcp.Description("Analysis mode: answer (default), summarize, or extract."),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			question, _ := args["question"].(string)
			mode, _ := args["mode"].(string)

			filesRaw, ok := args["files"].([]any)
			if !ok || len(filesRaw) == 0 {
				return mcp.NewToolResultError("files is required (array of absolute file paths)"), nil
			}

			files := make([]string, 0, len(filesRaw))
			for _, f := range filesRaw {
				path, ok := f.(string)
				if !ok {
					continue
				}
				path = strings.TrimSpace(path)
				if path != "" {
					files = append(files, path)
				}
			}
			if len(files) == 0 {
				return mcp.NewToolResultError("files must contain at least one valid file path"), nil
			}

			result := query.Query(ctx, dispatchPath, question, files, mode)
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
