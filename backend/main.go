package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/genai"
)

func main() {
	url := "http://localhost:8000"
	ctx := context.Background()

	log.Printf("Connecting to MCP server at %s", url)

	// Create an MCP client.
	mcpClient := mcp.NewClient(&mcp.Implementation{
		Name:    "time-client",
		Version: "1.0.0",
	}, nil)

	session, err := mcpClient.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func(session *mcp.ClientSession) {
		err := session.Close()
		if err != nil {
			log.Fatalf("Failed to close session: %v", err)
		}
	}(session)

	log.Printf("Connected to server (session ID: %s)", session.ID())

	log.Println("Listing available tools...")
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	declarations := make([]*genai.FunctionDeclaration, len(toolsResult.Tools))
	for i, tool := range toolsResult.Tools {
		fmt.Printf("Tool %d: %s\n", i+1, tool.Name)
		declarations[i] = &genai.FunctionDeclaration{
			Behavior:             "",
			Description:          tool.Description,
			Name:                 tool.Name,
			Parameters:           nil,
			ParametersJsonSchema: tool.InputSchema,
			Response:             nil,
			ResponseJsonSchema:   tool.OutputSchema,
		}
	}

	// GenAI
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: "AIzaSyAyDFy2fqYkJOVCbs7e1sj4hlrApb21PUE",
	})
	if err != nil {
		log.Fatal(err)
	}

	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{
				FunctionDeclarations: declarations,
			},
		},
		Temperature: genai.Ptr(float32(0.0)),
	}

	modelName := "gemini-2.5-flash"
	contents := []*genai.Content{
		{Parts: []*genai.Part{
			{Text: "What is the time in ny?"},
		},
			Role: "user"},
	}

	resp, err := client.Models.GenerateContent(ctx, modelName, contents, config)
	if err != nil {
		fmt.Errorf("failed to generate content: %w", err)
	}

	var funcCall *genai.FunctionCall
	for _, p := range resp.Candidates[0].Content.Parts {
		if p.FunctionCall != nil {
			funcCall = p.FunctionCall
			fmt.Print("The model suggests to call the function ")
			fmt.Printf("%q with args: %v\n", funcCall.Name, funcCall.Args)
			// Example response:
			// The model suggests to call the function "getCurrentWeather" with args: map[location:Boston]
		}
	}
	if funcCall == nil {
		log.Fatal("model did not suggest a function call")
	}

	// Use synthetic data to simulate a response from the external API.
	// In a real application, this would come from an actual weather API.
	res, _ := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      funcCall.Name,
		Arguments: funcCall.Args,
	})

	for _, content := range res.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			log.Printf("  %s", textContent.Text)
		}
	}
	funcResp := &genai.FunctionResponse{
		Name:     "getCurrentWeather",
		Response: map[string]any{"text": res.Content[0].(*mcp.TextContent).Text},
	}

	// Return conversation turns and API response to complete the model's response.
	contents = []*genai.Content{
		{Parts: []*genai.Part{
			{Text: "What is the time in ny?"},
		},
			Role: "user"},
		{Parts: []*genai.Part{
			{FunctionCall: funcCall},
		}},
		{Parts: []*genai.Part{
			{FunctionResponse: funcResp},
		}},
	}

	resp, err = client.Models.GenerateContent(ctx, modelName, contents, config)
	if err != nil {
		fmt.Errorf("failed to generate content: %w", err)
	}

	respText := resp.Text()

	fmt.Println(respText)

}

func runClient(url string) {
	ctx := context.Background()

	// Create the URL for the server.
	log.Printf("Connecting to MCP server at %s", url)

	// Create an MCP client.
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "time-client",
		Version: "1.0.0",
	}, nil)

	// Connect to the server.
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer session.Close()

	log.Printf("Connected to server (session ID: %s)", session.ID())

	// First, list available tools.
	log.Println("Listing available tools...")
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	for _, tool := range toolsResult.Tools {
		log.Printf("  - %s: %s\n", tool.Name, tool.Description)
		jsonBytes, err := json.MarshalIndent(tool, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal json: %v", err)
		}
		log.Printf(string(jsonBytes))
	}

	// Call the cityTime tool for each city.
	cities := []string{"nyc", "sf", "boston"}

	log.Println("Getting time for each city...")
	for _, city := range cities {
		// Call the tool.
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "cityTime",
			Arguments: map[string]any{
				"city": city,
			},
		})
		if err != nil {
			log.Printf("Failed to get time for %s: %v\n", city, err)
			continue
		}

		// Print the result.
		for _, content := range result.Content {
			if textContent, ok := content.(*mcp.TextContent); ok {
				log.Printf("  %s", textContent.Text)
			}
		}
	}

	log.Println("Client completed successfully")

}
