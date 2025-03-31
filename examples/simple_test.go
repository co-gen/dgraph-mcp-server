package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/dgraph-io/dgo/v2"
	"github.com/dgraph-io/dgo/v2/protos/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// A simple test script to verify Dgraph connection and operations
func main() {
	// Connect to Dgraph
	dgraphHost := getEnv("DGRAPH_HOST", "localhost:9080")
	conn, err := grpc.Dial(dgraphHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Dgraph: %v", err)
	}
	defer conn.Close()
	
	dgraphClient := dgo.NewDgraphClient(api.NewDgraphClient(conn))
	
	// Test connection
	ctx := context.Background()
	
	// 1. Set schema
	fmt.Println("Setting up schema...")
	schema := `
		name: string @index(exact) .
		age: int .
		friend: [uid] .
	`
	
	op := &api.Operation{
		Schema: schema,
	}
	
	err = dgraphClient.Alter(ctx, op)
	if err != nil {
		log.Fatalf("Schema alteration failed: %v", err)
	}
	fmt.Println("Schema set up successfully")
	
	// 2. Add some data
	fmt.Println("Adding sample data...")
	txn := dgraphClient.NewTxn()
	
	mutation := `
		_:alice <name> "Alice" .
		_:alice <age> "30" .
		_:bob <name> "Bob" .
		_:bob <age> "32" .
		_:alice <friend> _:bob .
	`
	
	mu := &api.Mutation{
		SetNquads: []byte(mutation),
		CommitNow: true,
	}
	
	_, err = txn.Mutate(ctx, mu)
	if err != nil {
		log.Fatalf("Mutation failed: %v", err)
	}
	txn.Discard(ctx)
	fmt.Println("Sample data added successfully")
	
	// 3. Query the data
	fmt.Println("Querying data...")
	txn = dgraphClient.NewTxn()
	
	query := `{
		people(func: has(name)) {
			name
			age
			friend {
				name
				age
			}
		}
	}`
	
	resp, err := txn.Query(ctx, query)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	txn.Discard(ctx)
	
	fmt.Println("Query result:")
	fmt.Println(string(resp.Json))
	
	fmt.Println("Test completed successfully!")
}

// Helper function to get environment variable with default fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
