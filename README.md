# Dgraph MCP Server

A Model Context Protocol (MCP) server implementation for Dgraph graph database, built using the [mcp-go](https://github.com/mark3labs/mcp-go) library.

## Overview

This project implements an MCP server that allows LLM applications to interact with a Dgraph database. It provides tools for:

- Executing DQL queries
- Performing mutations
- Altering the schema
- Retrieving the current schema

## Prerequisites

- Go 1.18 or higher
- Dgraph database (running locally or remotely)

## Installation

1. Clone this repository
2. Install dependencies:

```bash
go mod download
```

## Configuration

The server can be configured using environment variables:

- `DGRAPH_HOST`: Dgraph host address (default: `localhost:9080`)

## Usage

### Running the Server

```bash
go run main.go
```

The server uses standard input/output for communication with LLM applications.

### Available Tools

#### 1. dgraph_query

Execute a DQL query against Dgraph.

Parameters:
- `query` (string, required): The DQL query to execute
- `variables` (object, optional): Variables for the query

Example:
```json
{
  "tool": "dgraph_query",
  "params": {
    "query": "{ me(func: has(name)) { name } }"
  }
}
```

#### 2. dgraph_mutate

Execute a mutation against Dgraph.

Parameters:
- `mutation` (string, required): The RDF mutation to execute
- `commit` (boolean, optional): Whether to commit the transaction (default: true)

Example:
```json
{
  "tool": "dgraph_mutate",
  "params": {
    "mutation": "_:person <name> \"John Doe\" .",
    "commit": true
  }
}
```

#### 3. dgraph_alter_schema

Alter the Dgraph schema.

Parameters:
- `schema` (string, required): The schema definition to apply

Example:
```json
{
  "tool": "dgraph_alter_schema",
  "params": {
    "schema": "name: string @index(exact) ."
  }
}
```

### Available Resources

#### 1. dgraph://schema

Returns the current Dgraph schema.

## Integration with LLM Applications

This server can be integrated with any LLM application that supports the Model Context Protocol (MCP). The server communicates via standard input/output, making it easy to integrate with various LLM frameworks.

## Example Queries

### Basic Query

```
{
  people(func: has(name)) {
    name
    age
    friends {
      name
    }
  }
}
```

### Adding Data

```
_:alice <name> "Alice" .
_:alice <age> "30" .
_:bob <name> "Bob" .
_:bob <age> "32" .
_:alice <friend> _:bob .
```

### Schema Definition

```
name: string @index(exact) .
age: int .
friend: [uid] .
```

## License

MIT
