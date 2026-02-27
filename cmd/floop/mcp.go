package main

import (
	"context"
	"fmt"

	"github.com/nvandessel/floop/internal/mcp"
	"github.com/spf13/cobra"
)

func newMCPServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp-server",
		Short: "Run floop as an MCP (Model Context Protocol) server",
		Long: `Start an MCP server that exposes floop functionality over stdio.

The MCP server allows AI tools (Continue.dev, Cursor, Cline, Windsurf, GitHub Copilot)
to invoke floop tools directly:

  • floop_active  - Get active behaviors for current context
  • floop_learn   - Capture corrections and extract behaviors
  • floop_list    - List all behaviors or corrections

The server communicates via JSON-RPC 2.0 over stdin/stdout, following the
Model Context Protocol specification.

Configuration examples for each AI tool can be found in:
  docs/integrations/mcp-server.md

Example usage in Continue.dev config.json:

  {
    "mcpServers": {
      "floop": {
        "command": "floop",
        "args": ["mcp-server"],
        "cwd": "${workspaceFolder}"
      }
    }
  }
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, _ := cmd.Flags().GetString("root")

			// Create MCP server
			server, err := mcp.NewServer(&mcp.Config{
				Name:    "floop",
				Version: version,
				Root:    root,
			})
			if err != nil {
				return fmt.Errorf("failed to create MCP server: %w", err)
			}

			// Run server (blocks until client disconnects or SIGTERM/SIGINT)
			if err := server.Run(context.Background()); err != nil {
				return fmt.Errorf("MCP server error: %w", err)
			}

			return nil
		},
	}

	return cmd
}
