package main

import (
	"fmt"

	// ensure dependencies are tracked
	_ "github.com/google/go-dap"
	_ "github.com/mark3labs/mcp-go/server"
)

func main() {
	fmt.Println("lldb-debug-mcp server")
}
