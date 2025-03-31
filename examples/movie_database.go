package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dgraph-io/dgo/v2"
	"github.com/dgraph-io/dgo/v2/protos/api"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Example MCP server for a movie database using Dgraph
func main() {
	// Connect to Dgraph
	conn, err := grpc.Dial("localhost:9080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Dgraph: %v", err)
	}
	dgraphClient := dgo.NewDgraphClient(api.NewDgraphClient(conn))
	
	// Set up schema for movies database
	setupMovieSchema(dgraphClient)

	// Create MCP server
	s := server.NewMCPServer(
		"Movie Database MCP Server",
		"1.0.0",
	)

	// Add movie search tool
	searchTool := mcp.NewTool("search_movies",
		mcp.WithDescription("Search for movies by title, actor, director, or genre"),
		mcp.WithString("search_term",
			mcp.Required(),
			mcp.Description("The search term to look for in movie titles, actor names, etc."),
		),
		mcp.WithString("search_type",
			mcp.Description("Type of search: title, actor, director, or genre"),
			mcp.Enum("title", "actor", "director", "genre", "any"),
			mcp.Default("any"),
		),
	)

	// Add movie details resource template
	movieTemplate := mcp.NewResourceTemplate(
		"movies://{id}",
		"Movie Details",
		mcp.WithTemplateDescription("Returns detailed information about a specific movie"),
		mcp.WithTemplateMIMEType("application/json"),
	)

	// Add tools and resources with their handlers
	s.AddTool(searchTool, createMovieSearchHandler(dgraphClient))
	s.AddResourceTemplate(movieTemplate, createMovieDetailsHandler(dgraphClient))

	// Start the stdio server
	log.Println("Starting Movie Database MCP Server...")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// Set up the schema for our movie database
func setupMovieSchema(client *dgo.Dgraph) {
	ctx := context.Background()
	
	// Define schema
	schema := `
		title: string @index(fulltext) .
		release_year: int @index(int) .
		director: string @index(term) .
		actors: [string] @index(term) .
		genres: [string] @index(term) .
		rating: float .
		description: string .
		type Movie: string .
	`
	
	// Create operation
	op := &api.Operation{
		Schema: schema,
	}
	
	// Execute alter operation
	err := client.Alter(ctx, op)
	if err != nil {
		log.Fatalf("Schema alteration failed: %v", err)
	}
	
	log.Println("Movie schema set up successfully")
	
	// Add some sample data if the database is empty
	txn := client.NewTxn()
	defer txn.Discard(ctx)
	
	// Check if we already have movies
	resp, err := txn.Query(ctx, `{ movies(func: has(title)) { count(uid) } }`)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	
	// If no movies, add sample data
	if string(resp.Json) == "{\"movies\":[]}" {
		addSampleMovies(client)
	}
}

// Add some sample movies to the database
func addSampleMovies(client *dgo.Dgraph) {
	ctx := context.Background()
	txn := client.NewTxn()
	defer txn.Discard(ctx)
	
	// Sample movie data in RDF format
	mutation := `
		_:inception <title> "Inception" .
		_:inception <release_year> "2010" .
		_:inception <director> "Christopher Nolan" .
		_:inception <actors> "Leonardo DiCaprio" .
		_:inception <actors> "Joseph Gordon-Levitt" .
		_:inception <actors> "Ellen Page" .
		_:inception <genres> "Sci-Fi" .
		_:inception <genres> "Action" .
		_:inception <rating> "8.8" .
		_:inception <description> "A thief who steals corporate secrets through the use of dream-sharing technology is given the inverse task of planting an idea into the mind of a C.E.O." .
		_:inception <dgraph.type> "Movie" .
		
		_:darkKnight <title> "The Dark Knight" .
		_:darkKnight <release_year> "2008" .
		_:darkKnight <director> "Christopher Nolan" .
		_:darkKnight <actors> "Christian Bale" .
		_:darkKnight <actors> "Heath Ledger" .
		_:darkKnight <actors> "Aaron Eckhart" .
		_:darkKnight <genres> "Action" .
		_:darkKnight <genres> "Crime" .
		_:darkKnight <genres> "Drama" .
		_:darkKnight <rating> "9.0" .
		_:darkKnight <description> "When the menace known as the Joker wreaks havoc and chaos on the people of Gotham, Batman must accept one of the greatest psychological and physical tests of his ability to fight injustice." .
		_:darkKnight <dgraph.type> "Movie" .
		
		_:pulpFiction <title> "Pulp Fiction" .
		_:pulpFiction <release_year> "1994" .
		_:pulpFiction <director> "Quentin Tarantino" .
		_:pulpFiction <actors> "John Travolta" .
		_:pulpFiction <actors> "Uma Thurman" .
		_:pulpFiction <actors> "Samuel L. Jackson" .
		_:pulpFiction <genres> "Crime" .
		_:pulpFiction <genres> "Drama" .
		_:pulpFiction <rating> "8.9" .
		_:pulpFiction <description> "The lives of two mob hitmen, a boxer, a gangster and his wife, and a pair of diner bandits intertwine in four tales of violence and redemption." .
		_:pulpFiction <dgraph.type> "Movie" .
	`
	
	// Create mutation
	mu := &api.Mutation{
		SetNquads: []byte(mutation),
		CommitNow: true,
	}
	
	// Execute mutation
	_, err := txn.Mutate(ctx, mu)
	if err != nil {
		log.Fatalf("Mutation failed: %v", err)
	}
	
	log.Println("Sample movies added successfully")
}

// Create handler for the movie search tool
func createMovieSearchHandler(client *dgo.Dgraph) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		searchTerm, ok := request.Params.Arguments["search_term"].(string)
		if !ok {
			return nil, fmt.Errorf("search_term must be a string")
		}
		
		searchType := "any"
		if st, ok := request.Params.Arguments["search_type"].(string); ok {
			searchType = st
		}
		
		// Build query based on search type
		var query string
		switch searchType {
		case "title":
			query = fmt.Sprintf(`{
				movies(func: alloftext(title, "%s")) {
					uid
					title
					release_year
					director
					rating
				}
			}`, searchTerm)
		case "actor":
			query = fmt.Sprintf(`{
				movies(func: allofterms(actors, "%s")) {
					uid
					title
					release_year
					director
					actors
					rating
				}
			}`, searchTerm)
		case "director":
			query = fmt.Sprintf(`{
				movies(func: allofterms(director, "%s")) {
					uid
					title
					release_year
					director
					rating
				}
			}`, searchTerm)
		case "genre":
			query = fmt.Sprintf(`{
				movies(func: allofterms(genres, "%s")) {
					uid
					title
					release_year
					director
					genres
					rating
				}
			}`, searchTerm)
		default: // "any"
			query = fmt.Sprintf(`{
				movies(func: anyoftext(title director actors genres, "%s")) {
					uid
					title
					release_year
					director
					rating
				}
			}`, searchTerm)
		}
		
		// Execute query
		txn := client.NewTxn()
		defer txn.Discard(ctx)
		resp, err := txn.Query(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("query failed: %v", err)
		}
		
		// Return the JSON result
		return mcp.NewToolResultText(string(resp.Json)), nil
	}
}

// Create handler for the movie details resource
func createMovieDetailsHandler(client *dgo.Dgraph) func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// Extract movie ID from the URI
		// The URI format is "movies://{id}"
		uri := request.Params.URI
		if len(uri) < 9 { // "movies://" is 9 characters
			return nil, fmt.Errorf("invalid URI format: %s", uri)
		}
		
		movieID := uri[9:] // Extract the ID part
		
		// Query for movie details
		query := fmt.Sprintf(`{
			movie(func: uid(%s)) {
				title
				release_year
				director
				actors
				genres
				rating
				description
			}
		}`, movieID)
		
		// Execute query
		txn := client.NewTxn()
		defer txn.Discard(ctx)
		resp, err := txn.Query(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("query failed: %v", err)
		}
		
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(resp.Json),
			},
		}, nil
	}
}
