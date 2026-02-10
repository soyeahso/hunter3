# MCP Brave Search Plugin

An MCP (Model Context Protocol) server that provides web and news search capabilities using the Brave Search API.

## Features

- **Web Search**: Search the web and get formatted results with titles, URLs, and descriptions
- **News Search**: Search for recent news articles with source information
- Configurable result count (1-20 results)
- Country-specific search results
- Proper error handling and logging

## Tools Provided

### `brave_web_search`
Search the web using Brave Search API.

**Parameters:**
- `query` (required): The search query string
- `count` (optional): Number of results to return (1-20, default: 10)
- `country` (optional): Country code for search results (e.g., 'us', 'uk', 'ca', default: 'us')

**Example:**
```json
{
  "query": "Model Context Protocol",
  "count": 5,
  "country": "us"
}
```

### `brave_news_search`
Search for news articles using Brave Search API.

**Parameters:**
- `query` (required): The news search query string
- `count` (optional): Number of results to return (1-20, default: 10)
- `country` (optional): Country code for news results (e.g., 'us', 'uk', 'ca', default: 'us')

**Example:**
```json
{
  "query": "artificial intelligence",
  "count": 10,
  "country": "us"
}
```

## Setup

### 1. Get a Brave Search API Key

Sign up for a Brave Search API key at: https://brave.com/search/api/

### 2. Build the Plugin

From the hunter3 project root:

```bash
make all
```

This will build the binary to `dist/mcp-brave`

### 3. Configure Environment Variable

Add the Brave Search MCP server to your `.mcp.json` configuration:

```json
{
  "mcpServers": {
    "brave-search": {
      "command": "dist/mcp-brave",
      "args": [],
      "env": {
        "BRAVE_API_KEY": "your-actual-api-key-here"
      }
    }
  }
}
```

**Important**: Replace `your-actual-api-key-here` with your actual Brave Search API key.

### 4. Restart Your MCP Client

Restart your MCP client (e.g., Claude Desktop) to load the new server.

## Logging

Logs are written to `~/.hunter3/logs/mcp-brave.log`

## API Rate Limits

Be aware of Brave Search API rate limits based on your subscription tier. The free tier typically allows:
- 2,000 queries per month
- 1 query per second

## Example Usage

Once configured, you can use the tools in your MCP client:

**Web Search:**
```
Search for "golang best practices" using brave_web_search
```

**News Search:**
```
Find recent news about "space exploration" using brave_news_search
```

## Dependencies

- Go 1.25+
- github.com/mark3labs/mcp-go

## Integration with Hunter3

This plugin follows the Hunter3 MCP server conventions:
- Built to `dist/` directory
- Logs to `~/.hunter3/logs/`
- Uses standard MCP server initialization
- Properly advertises capabilities without `omitempty` on required fields
