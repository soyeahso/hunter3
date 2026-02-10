# Usage Examples - MCP Brave Search Plugin

## Tool Usage Examples

### Example 1: Basic Web Search

**Input:**
```json
{
  "name": "brave_web_search",
  "arguments": {
    "query": "Model Context Protocol"
  }
}
```

**Output:**
```
Web search results for: Model Context Protocol

1. Model Context Protocol | Anthropic
   URL: https://modelcontextprotocol.io
   Age: 2 months ago
   The Model Context Protocol (MCP) is an open protocol that standardizes how applications provide context to LLMs...

2. GitHub - modelcontextprotocol/servers
   URL: https://github.com/modelcontextprotocol/servers
   Age: 1 month ago
   Model Context Protocol Servers - A collection of reference implementations...

3. MCP Go SDK Documentation
   URL: https://github.com/mark3labs/mcp-go
   The Go implementation of the Model Context Protocol, enabling developers to build MCP servers...
```

### Example 2: Web Search with Country and Count

**Input:**
```json
{
  "name": "brave_web_search",
  "arguments": {
    "query": "best restaurants London",
    "count": 5,
    "country": "uk"
  }
}
```

**Output:**
```
Web search results for: best restaurants London

1. The 50 Best Restaurants in London - Time Out
   URL: https://www.timeout.com/london/restaurants/best-restaurants-in-london
   Age: 1 week ago
   Discover London's finest dining establishments, from Michelin-starred venues to hidden gems...

2. London's Top Restaurants 2026 - The Guardian
   URL: https://www.theguardian.com/food/london-restaurants
   Comprehensive guide to the best places to eat in London...

[... 3 more results ...]
```

### Example 3: News Search

**Input:**
```json
{
  "name": "brave_news_search",
  "arguments": {
    "query": "artificial intelligence breakthroughs"
  }
}
```

**Output:**
```
News search results for: artificial intelligence breakthroughs

1. New AI Model Achieves Human-Level Reasoning
   URL: https://example.com/ai-breakthrough-2026
   Source: Tech News Daily
   Age: 2 hours ago
   Researchers announce a significant breakthrough in artificial general intelligence...

2. OpenAI Releases GPT-5 with Enhanced Capabilities
   URL: https://example.com/gpt5-release
   Source: AI Weekly
   Age: 1 day ago
   The latest language model shows dramatic improvements in reasoning and code generation...

3. AI Helps Scientists Discover New Materials
   URL: https://example.com/ai-materials
   Source: Science Magazine
   Age: 3 days ago
   Machine learning algorithms accelerate the discovery of novel compounds...
```

### Example 4: Targeted News Search

**Input:**
```json
{
  "name": "brave_news_search",
  "arguments": {
    "query": "space exploration Mars",
    "count": 3,
    "country": "us"
  }
}
```

**Output:**
```
News search results for: space exploration Mars

1. NASA's Perseverance Rover Makes Major Discovery
   URL: https://nasa.gov/mars-discovery
   Source: NASA
   Age: 5 hours ago
   The Perseverance rover has found evidence of ancient microbial life on Mars...

2. SpaceX Plans First Crewed Mars Mission for 2028
   URL: https://spacex.com/mars-mission
   Source: SpaceX
   Age: 1 day ago
   Elon Musk announces ambitious timeline for sending humans to Mars...

3. International Mars Base Project Gains Momentum
   URL: https://example.com/mars-base
   Source: Space News
   Age: 2 days ago
   Multiple space agencies collaborate on plans for permanent Mars settlement...
```

## Natural Language Usage in MCP Client

When using the tools through an MCP client like Claude, you can use natural language:

### Example Conversations

**User:** "Can you search for information about the Model Context Protocol?"

**Assistant:** *Uses brave_web_search tool*
"I found several resources about the Model Context Protocol:

1. The official documentation at modelcontextprotocol.io explains that MCP is an open protocol...
2. There's a GitHub repository with reference implementations...
3. The Go SDK is available at..."

---

**User:** "What's in the news about AI today?"

**Assistant:** *Uses brave_news_search tool*
"Here are today's top AI news stories:

1. A major breakthrough in AI reasoning was announced just 2 hours ago...
2. OpenAI released GPT-5 yesterday with enhanced capabilities...
3. Scientists used AI to discover new materials..."

---

**User:** "Find me some restaurants in London"

**Assistant:** *Uses brave_web_search with country="uk"*
"I found several great restaurant guides for London:

1. Time Out lists the 50 best restaurants in London...
2. The Guardian has a comprehensive guide to London's dining scene...
[...]"

## Error Examples

### Missing API Key

**Error Log:**
```
2026/02/08 10:00:00 main.go:65: BRAVE_API_KEY environment variable not set
```

**Fix:** Add BRAVE_API_KEY to .mcp.json env section

### Invalid API Key

**Tool Response:**
```
Error: search failed: API error (status 401): {"message":"Invalid API key"}
```

**Fix:** Check that your API key is correct in .mcp.json

### Rate Limit Exceeded

**Tool Response:**
```
Error: search failed: API error (status 429): {"message":"Rate limit exceeded"}
```

**Fix:** Wait before making more requests, or upgrade your Brave API plan

### Empty Query

**Tool Response:**
```
Error: query parameter is required
```

**Fix:** Provide a non-empty query string

## Testing Commands

### Using echo and jq

```bash
# Test web search
echo '{
  "method": "tools/call",
  "params": {
    "name": "brave_web_search",
    "arguments": {
      "query": "golang best practices",
      "count": 5
    }
  }
}' | dist/mcp-brave

# Test news search
echo '{
  "method": "tools/call",
  "params": {
    "name": "brave_news_search",
    "arguments": {
      "query": "space exploration",
      "count": 3
    }
  }
}' | dist/mcp-brave
```

## API Response Structure (Internal)

For developers modifying the code, here's what Brave API returns:

### Web Search Response
```json
{
  "query": {
    "original": "golang best practices"
  },
  "web": {
    "results": [
      {
        "title": "Effective Go - The Go Programming Language",
        "url": "https://golang.org/doc/effective_go",
        "description": "A document that gives tips for writing clear...",
        "age": "Updated recently"
      }
    ]
  }
}
```

### News Search Response
```json
{
  "query": {
    "original": "space exploration"
  },
  "news": {
    "results": [
      {
        "title": "NASA Mars Discovery",
        "url": "https://nasa.gov/news",
        "description": "Major breakthrough on Mars...",
        "age": "2 hours ago",
        "source": "NASA"
      }
    ]
  }
}
```

## Performance Notes

- Average response time: 500-1500ms
- Depends on Brave API performance and network latency
- Results are returned as formatted text, not JSON
- Consider caching for frequently requested queries

## Best Practices

1. **Be Specific:** More specific queries yield better results
   - Good: "golang error handling best practices"
   - Bad: "golang"

2. **Use Country Codes:** For location-specific results
   - "restaurants Paris" with country="fr"

3. **Adjust Count:** Balance between comprehensiveness and speed
   - Use count=5 for quick overview
   - Use count=20 for thorough research

4. **News vs Web:** Choose the right tool
   - Use news_search for recent events
   - Use web_search for general information

5. **Rate Limits:** Be mindful of API limits
   - Free tier: 2,000 queries/month
   - Space out requests during development
