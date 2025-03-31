package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/dgraph-io/dgo/v2"
	"github.com/dgraph-io/dgo/v2/protos/api"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Default Dgraph connection settings
const (
	defaultDgraphHost = "localhost:9080"
)

func main() {
	// Get Dgraph connection settings from environment or use defaults
	dgraphHost := getEnv("DGRAPH_HOST", defaultDgraphHost)

	// Connect to Dgraph
	dgraphClient, err := connectToDgraph(dgraphHost)
	if err != nil {
		log.Fatalf("Failed to connect to Dgraph: %v", err)
	}
	log.Printf("Connected to Dgraph at %s", dgraphHost)

	// Create MCP server
	s := server.NewMCPServer(
		"Dgraph MCP Server",
		"1.0.0",
	)

	// Add query tool
	queryTool := mcp.NewTool("dgraph_query",
		mcp.WithDescription("Execute a GraphQL+- query against Dgraph"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The GraphQL+- query to execute"),
		),
		mcp.WithObject("variables",
			mcp.Description("Variables for the query (optional)"),
		),
	)

	// Add mutation tool
	mutationTool := mcp.NewTool("dgraph_mutate",
		mcp.WithDescription("Execute a mutation against Dgraph"),
		mcp.WithString("mutation",
			mcp.Required(),
			mcp.Description("The RDF mutation to execute"),
		),
		mcp.WithBoolean("commit",
			mcp.Description("Whether to commit the transaction (default: true)"),
		),
	)

	// Add schema tool
	schemaTool := mcp.NewTool("dgraph_alter_schema",
		mcp.WithDescription("Alter the Dgraph schema"),
		mcp.WithString("schema",
			mcp.Required(),
			mcp.Description("The schema definition to apply"),
		),
	)

	// Add tools with their handlers
	s.AddTool(queryTool, createQueryHandler(dgraphClient))
	s.AddTool(mutationTool, createMutationHandler(dgraphClient))
	s.AddTool(schemaTool, createSchemaHandler(dgraphClient))

	// Add schema resource
	schemaResource := mcp.NewResource(
		"dgraph://schema",
		"Dgraph Schema",
		mcp.WithResourceDescription("The current Dgraph schema"),
		mcp.WithMIMEType("text/plain"),
	)

	// Add resource with its handler
	s.AddResource(schemaResource, createSchemaResourceHandler(dgraphClient))

	// Start the stdio server
	log.Println("Starting Dgraph MCP Server...")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// Helper function to get environment variable with default fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// Connect to Dgraph
func connectToDgraph(host string) (*dgo.Dgraph, error) {
	conn, err := grpc.Dial(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return dgo.NewDgraphClient(
		api.NewDgraphClient(conn),
	), nil
}

// Create handler for the query tool
func createQueryHandler(client *dgo.Dgraph) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, ok := request.Params.Arguments["query"].(string)
		if !ok {
			return nil, fmt.Errorf("query must be a string")
		}

		// Create transaction
		txn := client.NewTxn()
		defer txn.Discard(ctx)

		// Execute query
		resp, err := txn.Query(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("query failed: %v", err)
		}

		// Return the JSON result
		return mcp.NewToolResultText(string(resp.Json)), nil
	}
}

// Create handler for the mutation tool
func createMutationHandler(client *dgo.Dgraph) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mutation, ok := request.Params.Arguments["mutation"].(string)
		if !ok {
			return nil, fmt.Errorf("mutation must be a string")
		}

		// Default to committing the transaction
		commit := true
		if commitArg, ok := request.Params.Arguments["commit"].(bool); ok {
			commit = commitArg
		}

		// Create transaction
		txn := client.NewTxn()
		defer txn.Discard(ctx)

		// Create mutation
		mu := &api.Mutation{
			SetNquads: []byte(mutation),
			CommitNow: commit,
		}

		// Execute mutation
		resp, err := txn.Mutate(ctx, mu)
		if err != nil {
			return nil, fmt.Errorf("mutation failed: %v", err)
		}

		// Return the JSON result
		return mcp.NewToolResultText(fmt.Sprintf("Mutation successful. Response: %v", resp)), nil
	}
}

// Create handler for the schema tool
func createSchemaHandler(client *dgo.Dgraph) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		schema, ok := request.Params.Arguments["schema"].(string)
		if !ok {
			return nil, fmt.Errorf("schema must be a string")
		}

		// Create operation
		op := &api.Operation{
			Schema: schema,
		}

		// Execute alter operation
		err := client.Alter(ctx, op)
		if err != nil {
			return nil, fmt.Errorf("schema alteration failed: %v", err)
		}

		return mcp.NewToolResultText("Schema updated successfully"), nil
	}
}

// Create handler for the schema resource
func createSchemaResourceHandler(client *dgo.Dgraph) func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// Create operation to get schema
		op := &api.Operation{
			Schema: "",
		}

		// Execute operation
		resp, err := client.NewTxn().Query(ctx, "schema {}")
		if err != nil {
			return nil, fmt.Errorf("failed to get schema: %v", err)
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "dgraph://schema",
				MIMEType: "text/plain",
				Text:     string(resp.Json),
			},
		}, nil
	}
}
