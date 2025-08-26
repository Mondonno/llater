package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ollama/ollama/api"
	"github.com/spf13/cobra"
)

const (
	RoleUser       = "user"
	RoleSystem     = "system"
	RoleChallenger = "challenger"
	RoleDefender   = "defender"
	RoleSummarizer = "summarizer"

	MaxHistory = 10 // max number of previous messages to keep
)

type Message struct {
	Role    string
	Content string
}

type DebateRound struct {
	Challenger string
	Defender   string
}

type DebateConfig struct {
	Rounds     int
	Duration   string
	InputFile  string
	OutputFile string
}

type LLMConfig struct {
	ChallengerModel string
	DefenderModel   string
	ChalPrompt      string
	DefPrompt       string
}

// Interface for LLM client to allow mocking
type LLMClient interface {
	Generate(ctx context.Context, model, prompt string) (string, error)
}

// ---------------- Main -----------------
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cmd := &cobra.Command{
		Use:   "llmdebate",
		Short: "Run context-preserving LLM debates",
		RunE:  runCLI,
	}
	addFlags(cmd)
	return cmd.Execute()
}

func addFlags(cmd *cobra.Command) {
	cmd.Flags().String("input", "", "Path to input notes")
	cmd.Flags().String("output", "", "Path to output report")
	cmd.Flags().Int("rounds", 0, "Number of debate rounds")
	cmd.Flags().String("duration", "", "Total duration (e.g., 1h)")
	cmd.Flags().String("challenger", "llama3", "Challenger model")
	cmd.Flags().String("defender", "llama3", "Defender model")
	cmd.Flags().String("challenger-prompt", "", "Challenger prompt or file")
	cmd.Flags().String("defender-prompt", "", "Defender prompt or file")
}

// ---------------- CLI -----------------
func runCLI(cmd *cobra.Command, args []string) error {
	config, llm, err := parseFlags(cmd)
	if err != nil {
		return err
	}

	if config.Rounds > 0 && config.Duration != "" {
		return errors.New("--rounds and --duration cannot be used together")
	}

	input, err := loadInput(config.InputFile)
	if err != nil {
		return err
	}

	client, err := api.ClientFromEnvironment()
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	llmClient := NewOllamaClient(client)

	if config.Rounds == 0 && config.Duration != "" {
		config.Rounds, err = estimateRounds(llmClient, input, llm, config.Duration)
		if err != nil {
			return err
		}
	}
	if config.Rounds <= 0 {
		config.Rounds = 3
	}

	debate, err := runDebateFlow(llmClient, input, config, llm)
	if err != nil {
		return err
	}

	report, err := summarizeDebate(llmClient, debate, llm.ChallengerModel)
	if err != nil {
		return err
	}

	return os.WriteFile(config.OutputFile, []byte(report), 0o644)
}

// ---------------- Ollama wrapper -----------------
type OllamaClient struct {
	Client *api.Client
}

func NewOllamaClient(client *api.Client) *OllamaClient {
	return &OllamaClient{Client: client}
}

func (o *OllamaClient) Generate(ctx context.Context, model, prompt string) (string, error) {
	var result string
	err := o.Client.Generate(ctx, &api.GenerateRequest{
		Model:  model,
		Prompt: prompt,
	}, func(resp api.GenerateResponse) error {
		result += resp.Response
		return nil
	})
	if err != nil {
		return "", err
	}
	if result == "" {
		return "", errors.New("empty response")
	}
	return result, nil
}

// ---------------- Debate -----------------
func runDebateFlow(client LLMClient, claim string, config DebateConfig, llm LLMConfig) ([]DebateRound, error) {
	var rounds []DebateRound
	history := []Message{{Role: RoleUser, Content: claim}}

	for i := 0; i < config.Rounds; i++ {
		history = trimHistory(history, MaxHistory)

		chal, err := runSingleRound(client, llm.ChallengerModel, llm.ChalPrompt, history, RoleChallenger)
		if err != nil {
			return nil, err
		}
		history = append(history, Message{Role: RoleChallenger, Content: chal})

		def, err := runSingleRound(client, llm.DefenderModel, llm.DefPrompt, history, RoleDefender)
		if err != nil {
			return nil, err
		}
		history = append(history, Message{Role: RoleDefender, Content: def})

		rounds = append(rounds, DebateRound{Challenger: chal, Defender: def})
	}
	return rounds, nil
}

func trimHistory(history []Message, max int) []Message {
	if len(history) <= max {
		return history
	}
	return history[len(history)-max:]
}

func runSingleRound(client LLMClient, model, prompt string, history []Message, role string) (string, error) {
	fullPrompt := prompt + "\n"
	for _, m := range history {
		fullPrompt += fmt.Sprintf("%s: %s\n", m.Role, m.Content)
	}
	return client.Generate(context.Background(), model, fullPrompt)
}

// ---------------- Summarize -----------------
func summarizeDebate(client LLMClient, debate []DebateRound, model string) (string, error) {
	var fullText string
	for i, r := range debate {
		fullText += fmt.Sprintf("### Round %d\nChallenger: %s\nDefender: %s\n\n", i+1, r.Challenger, r.Defender)
	}
	return runSingleRound(client, model, "Summarize the debate: top blind spots, opportunities, deadly assumption.", []Message{{Role: RoleSystem, Content: fullText}}, RoleSummarizer)
}

// ---------------- Utils -----------------
func parseFlags(cmd *cobra.Command) (DebateConfig, LLMConfig, error) {
	input, _ := cmd.Flags().GetString("input")
	output, _ := cmd.Flags().GetString("output")
	if input == "" || output == "" {
		return DebateConfig{}, LLMConfig{}, errors.New("input and output required")
	}
	rounds, _ := cmd.Flags().GetInt("rounds")
	duration, _ := cmd.Flags().GetString("duration")
	chalModel, _ := cmd.Flags().GetString("challenger")
	defModel, _ := cmd.Flags().GetString("defender")
	chalPrompt, _ := cmd.Flags().GetString("challenger-prompt")
	defPrompt, _ := cmd.Flags().GetString("defender-prompt")

	config := DebateConfig{Rounds: rounds, Duration: duration, InputFile: input, OutputFile: output}
	llm := LLMConfig{
		ChallengerModel: chalModel,
		DefenderModel:   defModel,
		ChalPrompt:      mustLoadPrompt(chalPrompt, "You are the Challenger. Attack ruthlessly:"),
		DefPrompt:       mustLoadPrompt(defPrompt, "You are the Defender. Represent the user:"),
	}
	return config, llm, nil
}

func mustLoadPrompt(path, fallback string) string {
	if path == "" {
		return fallback
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ Warning: failed to load prompt %s, using default\n", path)
		return fallback
	}
	return string(data)
}

func loadInput(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading input file: %w", err)
	}
	return string(data), nil
}

func estimateRounds(client LLMClient, claim string, llm LLMConfig, duration string) (int, error) {
	start := time.Now()
	_, err := runDebateFlow(client, claim, DebateConfig{Rounds: 1}, llm)
	if err != nil {
		return 0, err
	}
	elapsed := time.Since(start)
	total, err := time.ParseDuration(duration)
	if err != nil {
		return 0, fmt.Errorf("invalid duration: %w", err)
	}
	rounds := int(total.Seconds() / elapsed.Seconds())
	if rounds < 1 {
		rounds = 1
	}
	fmt.Printf("Estimated %d rounds (1 round = %v)\n", rounds, elapsed)
	return rounds, nil
}
