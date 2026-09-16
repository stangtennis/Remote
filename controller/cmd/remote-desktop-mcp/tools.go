package main

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Tool allow-list. Support and device tools remain read-only; knowledge writes
// are forwarded only to the central admin-gated MCP endpoint.
const (
	toolListDevices        = "list_devices"
	toolStatus             = "status"
	toolSupportList        = "support_list"
	toolSupportWatchStatus = "support_watch_status"
	toolSupportContext     = "support_context"
	toolClientHistory      = "client_history"
	toolKnowledgeSearch    = "knowledge_search"
	toolKnowledgeDraft     = "knowledge_draft"
	toolKnowledgePublish   = "knowledge_publish"
)

// toolAllowList is the single source of truth for registered tool names.
var toolAllowList = []string{
	toolListDevices,
	toolStatus,
	toolSupportList,
	toolSupportWatchStatus,
	toolSupportContext,
	toolClientHistory,
	toolKnowledgeSearch,
	toolKnowledgeDraft,
	toolKnowledgePublish,
}

// newServer builds the MCP server with the bounded support and knowledge tools.
func newServer(run *cliRunner, contextClients ...*edgeMCPClient) *server.MCPServer {
	var contextClient *edgeMCPClient
	if len(contextClients) > 0 {
		contextClient = contextClients[0]
	}
	s := server.NewMCPServer(
		"remote-desktop-mcp",
		"0.1.0",
		// Never let a tool panic kill the stdio transport.
		server.WithRecovery(),
	)

	s.AddTool(
		mcp.NewTool(toolListDevices, readOnlyToolDescription("List remote desktop devices registered for the configured CLI account. Returns structured JSON parsed from `remote-desktop-cli list` output.")),
		handleListDevices(run, contextClient),
	)
	s.AddTool(
		mcp.NewTool(toolStatus, readOnlyToolDescription("Report local remote-desktop daemon connection status from `remote-desktop-cli status`. Reports daemon-absent as plain text.")),
		handleStatus(run),
	)
	s.AddTool(
		mcp.NewTool(toolSupportList, readOnlyToolDescription("List active AI support client sessions (short key, status, expiry, session id) from `remote-desktop-cli support-list`.")),
		handleSupportList(run),
	)
	s.AddTool(
		mcp.NewTool(toolSupportWatchStatus, readOnlyToolDescription("Report whether the same-user `remote-desktop-cli support-watch` watcher is running and the last bounded lines of its log. Never starts or stops the watcher.")),
		handleSupportWatchStatus(),
	)
	s.AddTool(
		mcp.NewTool(toolSupportContext, readOnlyToolDescription("Build the shared AI-support context for a client from the central MCP server: status, recent history, and optional published knowledge."), mcp.WithString("client_id", mcp.Required()), mcp.WithString("knowledge_query")),
		handleEdgeTool(contextClient, "support_context"),
	)
	s.AddTool(
		mcp.NewTool(toolClientHistory, readOnlyToolDescription("Read bounded, redacted history for a remote or persistent SSH AI-support client from the central MCP server."), mcp.WithString("client_id", mcp.Required())),
		handleEdgeTool(contextClient, "client_history"),
	)
	s.AddTool(
		mcp.NewTool(toolKnowledgeSearch, readOnlyToolDescription("Search the central published AI-support knowledge base."), mcp.WithString("query", mcp.Required()), mcp.WithString("category")),
		handleEdgeTool(contextClient, "knowledge_search"),
	)
	s.AddTool(
		mcp.NewTool(toolKnowledgeDraft, knowledgeWriteToolDescription("Save a support knowledge draft in the central knowledge base. It remains unpublished and requires admin approval."), mcp.WithString("title", mcp.Required()), mcp.WithString("content", mcp.Required()), mcp.WithString("summary"), mcp.WithString("category"), mcp.WithArray("tags"), mcp.WithString("source")),
		handleEdgeTool(contextClient, "knowledge_draft"),
	)
	s.AddTool(
		mcp.NewTool(toolKnowledgePublish, knowledgeWriteToolDescription("Publish an existing support knowledge draft. Requires admin approval."), mcp.WithString("id", mcp.Required())),
		handleEdgeTool(contextClient, "knowledge_publish"),
	)
	return s
}

func readOnlyToolDescription(description string) mcp.ToolOption {
	return func(tool *mcp.Tool) {
		mcp.WithDescription(description)(tool)
		mcp.WithReadOnlyHintAnnotation(true)(tool)
		mcp.WithDestructiveHintAnnotation(false)(tool)
		mcp.WithIdempotentHintAnnotation(true)(tool)
	}
}

func knowledgeWriteToolDescription(description string) mcp.ToolOption {
	return func(tool *mcp.Tool) {
		mcp.WithDescription(description)(tool)
		mcp.WithReadOnlyHintAnnotation(false)(tool)
		mcp.WithDestructiveHintAnnotation(false)(tool)
		mcp.WithIdempotentHintAnnotation(false)(tool)
	}
}

func handleListDevices(run *cliRunner, contextClient *edgeMCPClient) server.ToolHandlerFunc {
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if contextClient != nil {
			if text, err := contextClient.callTool(context.Background(), "list_clients", map[string]any{}); err == nil {
				return mcp.NewToolResultText(text), nil
			}
		}
		stdout, stderr, err := run.runCLI("list")
		if err != nil {
			return mcp.NewToolResultErrorf("list failed: %v%s", err, stderrSuffix(stderr)), nil
		}
		if entries, ok := parseDevicesList(stdout); ok && len(entries) > 0 {
			if data, jsonErr := marshalJSON(entries); jsonErr == nil {
				return mcp.NewToolResultText(string(data)), nil
			}
		}
		// Conservative fallback: bounded, redacted raw CLI text.
		return mcp.NewToolResultText(nonEmpty(stdout, "No devices registered.")), nil
	}
}

func handleStatus(run *cliRunner) server.ToolHandlerFunc {
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stdout, stderr, err := run.runCLI("status")
		if err != nil {
			return mcp.NewToolResultErrorf("status failed: %v%s", err, stderrSuffix(stderr)), nil
		}
		return mcp.NewToolResultText(nonEmpty(stdout, "Not connected (daemon not running)")), nil
	}
}

func handleSupportList(run *cliRunner) server.ToolHandlerFunc {
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stdout, stderr, err := run.runCLI("support-list")
		if err != nil {
			return mcp.NewToolResultErrorf("support-list failed: %v%s", err, stderrSuffix(stderr)), nil
		}
		if entries, ok := parseSupportList(stdout); ok && len(entries) > 0 {
			if data, jsonErr := marshalJSON(entries); jsonErr == nil {
				return mcp.NewToolResultText(string(data)), nil
			}
		}
		return mcp.NewToolResultText(nonEmpty(stdout, "No active AI support clients.")), nil
	}
}

func handleSupportWatchStatus() server.ToolHandlerFunc {
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(supportWatchStatusText()), nil
	}
}

func handleEdgeTool(client *edgeMCPClient, name string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if client == nil {
			return mcp.NewToolResultError("central MCP context is unavailable"), nil
		}
		text, err := client.callTool(ctx, name, request.GetArguments())
		if err != nil {
			return mcp.NewToolResultErrorf("%s failed: %v", name, err), nil
		}
		return mcp.NewToolResultText(text), nil
	}
}

func stderrSuffix(stderr string) string {
	if stderr == "" {
		return ""
	}
	return "\n" + stderr
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
