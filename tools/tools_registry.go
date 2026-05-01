package tools

import (
	"github.com/mark3labs/mcp-go/server"
)

func RegisterAllTools(s *server.MCPServer) {
	RegisterTools(s)
	registerMoreTools(s)
	registerEvenMoreTools(s)
	registerEvenMoreMoreTools(s)
}
