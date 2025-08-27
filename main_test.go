package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

type call struct {
	model  string
	system string
	prompt string
}

type fakeLLM struct {
	calls   []call
	respond func(model, system, prompt string) (string, error)
}

func (f *fakeLLM) Generate(ctx context.Context, model, system, prompt string) (string, error) {
	f.calls = append(f.calls, call{model: model, system: system, prompt: prompt})
	if f.respond != nil {
		return f.respond(model, system, prompt)
	}
	return "ok", nil
}

func TestTrimHistory(t *testing.T) {
	h := []Message{}
	for i := 0; i < 15; i++ {
		h = append(h, Message{Role: "r", Content: "c"})
	}
	out := trimHistory(h, 10)
	if len(out) != 10 {
		t.Fatalf("expected 10, got %d", len(out))
	}
}

func TestRunSingleRound_AssemblesPromptAndUsesParams(t *testing.T) {
	fake := &fakeLLM{respond: func(model, system, prompt string) (string, error) {
		return "resp", nil
	}}
	history := []Message{{Role: RoleUser, Content: "claim"}, {Role: RoleChallenger, Content: "c1"}}
	res, err := runSingleRound(fake, "model-x", "sys-y", history)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "resp" {
		t.Fatalf("unexpected response: %q", res)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fake.calls))
	}
	c := fake.calls[0]
	if c.model != "model-x" || c.system != "sys-y" {
		t.Fatalf("unexpected model/system: %q / %q", c.model, c.system)
	}
	expectedPrompt := "user: claim\nchallenger: c1\n"
	if c.prompt != expectedPrompt {
		t.Fatalf("unexpected prompt. want %q, got %q", expectedPrompt, c.prompt)
	}
}

func TestRunDebateFlow_Basic(t *testing.T) {
	fake := &fakeLLM{respond: func(model, system, prompt string) (string, error) {
		if strings.HasPrefix(system, RoleChallenger) {
			return "chal", nil
		}
		if strings.HasPrefix(system, RoleDefender) {
			return "def", nil
		}
		return "?", nil
	}}
	cfg := DebateConfig{Rounds: 2}
	llm := LLMConfig{ChallengerModel: "mC", DefenderModel: "mD", ChalPrompt: "CP", DefPrompt: "DP"}
	rounds, err := runDebateFlow(fake, "the claim", cfg, llm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rounds) != 2 {
		t.Fatalf("expected 2 rounds, got %d", len(rounds))
	}
	if rounds[0].Challenger != "chal" || rounds[0].Defender != "def" {
		t.Fatalf("unexpected round[0]: %+v", rounds[0])
	}
	if len(fake.calls) != 4 {
		t.Fatalf("expected 4 calls, got %d", len(fake.calls))
	}
	if !strings.HasPrefix(fake.calls[0].system, RoleChallenger) || !strings.HasPrefix(fake.calls[1].system, RoleDefender) {
		t.Fatalf("unexpected call order: %#v %#v", fake.calls[0].system, fake.calls[1].system)
	}
	// Third call (round 2 challenger) should include previous chal/def in prompt history
	c3 := fake.calls[2]
	if strings.Count(c3.prompt, RoleChallenger+":") < 1 || strings.Count(c3.prompt, RoleDefender+":") < 1 {
		t.Fatalf("expected previous round in history, got: %q", c3.prompt)
	}
	// All prompts should start with the user claim
	for i, c := range fake.calls {
		if !strings.HasPrefix(c.prompt, RoleUser+": the claim\n") {
			t.Fatalf("call %d prompt does not start with claim: %q", i, c.prompt)
		}
	}
}

func TestSummarizeDebate_ComposesText(t *testing.T) {
	fake := &fakeLLM{respond: func(model, system, prompt string) (string, error) { return "summary", nil }}
	deb := []DebateRound{{Challenger: "C", Defender: "D"}}
	out, err := summarizeDebate(fake, deb, "m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "summary" {
		t.Fatalf("unexpected summary: %q", out)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fake.calls))
	}
	c := fake.calls[0]
	if !strings.HasPrefix(c.system, "Summarize the debate:") {
		t.Fatalf("unexpected system prompt: %q", c.system)
	}
	if !strings.Contains(c.prompt, "### Round 1") || !strings.Contains(c.prompt, "Challenger: C") || !strings.Contains(c.prompt, "Defender: D") {
		t.Fatalf("summary input missing content: %q", c.prompt)
	}
}

func TestParseFlags_Valid(t *testing.T) {
	cmd := &cobra.Command{Use: "llmdebate"}
	addFlags(cmd)
	// create temp prompt files
	dir := t.TempDir()
	chalFile := filepath.Join(dir, "chal.txt")
	defFile := filepath.Join(dir, "def.txt")
	if err := os.WriteFile(chalFile, []byte("Custom Chal"), 0o644); err != nil {
		t.Fatalf("write chal file: %v", err)
	}
	if err := os.WriteFile(defFile, []byte("Custom Def"), 0o644); err != nil {
		t.Fatalf("write def file: %v", err)
	}
	_ = cmd.Flags().Set("input", "in.md")
	_ = cmd.Flags().Set("output", "out.md")
	_ = cmd.Flags().Set("rounds", "2")
	_ = cmd.Flags().Set("challenger", "mc")
	_ = cmd.Flags().Set("defender", "md")
	_ = cmd.Flags().Set("challenger-prompt", chalFile)
	_ = cmd.Flags().Set("defender-prompt", defFile)

	cfg, llm, err := parseFlags(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Rounds != 2 || cfg.InputFile != "in.md" || cfg.OutputFile != "out.md" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if llm.ChallengerModel != "mc" || llm.DefenderModel != "md" {
		t.Fatalf("unexpected llm models: %+v", llm)
	}
	if llm.ChalPrompt != "Custom Chal" || llm.DefPrompt != "Custom Def" {
		t.Fatalf("unexpected prompts: %+v", llm)
	}
}

func TestParseFlags_MissingRequired(t *testing.T) {
	cmd := &cobra.Command{Use: "llmdebate"}
	addFlags(cmd)
	_, _, err := parseFlags(cmd)
	if err == nil {
		t.Fatalf("expected error when input/output missing")
	}
}

func TestMustLoadPrompt_FallbackOnMissing(t *testing.T) {
	out := mustLoadPrompt("/path/does/not/exist", "FB")
	if out != "FB" {
		t.Fatalf("expected fallback, got %q", out)
	}
}

func TestLoadInput_FileAndMissing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "in.txt")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	s, err := loadInput(p)
	if err != nil || s != "hello" {
		t.Fatalf("unexpected load: %q, %v", s, err)
	}
	if _, err := loadInput(filepath.Join(dir, "missing.txt")); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestEstimateRounds_InvalidDuration(t *testing.T) {
	fake := &fakeLLM{}
	_, err := estimateRounds(fake, "claim", LLMConfig{}, "not-a-duration")
	if err == nil {
		t.Fatalf("expected error for invalid duration")
	}
}
