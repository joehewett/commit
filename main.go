package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

var debugMode bool

func debug(format string, a ...interface{}) {
	if debugMode {
		fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", a...)
	}
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
}

type AnthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

func getInput(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	var input string
	_, _ = fmt.Scanln(&input)
	return strings.ToLower(strings.TrimSpace(input))
}

func editMessage(initial string) (string, error) {
	// Create temporary file
	tmpfile, err := os.CreateTemp("", "commit-msg-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpfile.Name())

	// Write initial message to file
	if _, err := tmpfile.WriteString(initial); err != nil {
		return "", err
	}
	tmpfile.Close()

	// Open editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // fallback to vim
	}
	cmd := exec.Command(editor, tmpfile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	// Read edited message
	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func commitChanges(message string) error {
	debug("Running git commit")
	commitCmd := exec.Command("git", "commit", "-m", message)
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("error running git commit: %w", err)
	}

	return nil
}

func main() {
	// Remove dry-run flag, keep debug only
	flag.BoolVar(&debugMode, "debug", false, "Enable debug output")
	flag.Parse()

	// Check for API key
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: ANTHROPIC_API_KEY environment variable is not set")
		os.Exit(1)
	}

	// Get git diff for staged changes
	debug("Getting git diff for staged changes...")
	diffContext, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting git diff:", err)
		os.Exit(1)
	}
	debug("Diff length: %d bytes", len(diffContext))
	debug("Diff: %s", string(diffContext))

	// Get list of new staged files
	debug("Getting new staged files...")
	newFilesOutput, err := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=A").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting new staged files:", err)
		os.Exit(1)
	}
	newFiles := strings.Fields(string(newFilesOutput))
	debug("New staged files: %v", newFiles)

	// Check if there are any staged changes at all
	if len(diffContext) == 0 && len(newFiles) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No staged changes found")
		os.Exit(1)
	}

	// If there are new files, we need to get their content and add it to the diff
	if len(newFiles) > 0 {
		debug("Getting diff for new staged files...")
		for _, file := range newFiles {
			fileContent, err := os.ReadFile(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", file, err)
				os.Exit(1)
			}
			diffContent := fmt.Sprintf("\n--- /dev/null\n+++ b/%s\n%s", file, string(fileContent))
			diffContext = append(diffContext, []byte(diffContent)...)
		}
	}

	// Get recent commits
	debug("Getting recent commits...")
	recentCommits, err := exec.Command("git", "log", "-3", "--pretty=format:%B").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting recent commits:", err)
		os.Exit(1)
	}
	debug("Recent commits length: %d bytes", len(recentCommits))

	debug("Final diff: %s", string(diffContext))

	// Prepare prompt
	prompt := fmt.Sprintf(`Generate a git commit message following this structure:
1. First line: conventional commit format (type: concise description) (remember to use semantic types like feat, fix, docs, style, refactor, perf, test, chore, etc.)
2. Optional bullet points if more context helps:
   - Keep the second line blank
   - Keep them short and direct
   - Focus on what changed
   - Always be terse
   - Don't overly explain
   - Drop any fluffy or formal language

Return ONLY the commit message - no introduction, no explanation, no quotes around it.

Examples:
feat: add user auth system

- Add JWT tokens for API auth
- Handle token refresh for long sessions

fix: resolve memory leak in worker pool

- Clean up idle connections
- Add timeout for stale workers

Simple change example:
fix: typo in README.md

Very important: Do not respond with any of the examples. Your message must be based off the diff that is about to be provided, with a little bit of styling informed by the recent commits you're about to see.

Recent commits from this repo (for style reference):
%s

Here's the current diff. Your commit message should be based off this diff:

%s`, string(recentCommits), string(diffContext))

	// Prepare request
	reqBody := AnthropicRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 300,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error marshaling request:", err)
		os.Exit(1)
	}

	debug("Sending request to Anthropic API...")
	// Make API request
	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating request:", err)
		os.Exit(1)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error making request:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error reading response:", err)
		os.Exit(1)
	}

	debug("Received response from API")
	// Parse response
	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing response:", err)
		os.Exit(1)
	}

	if len(anthropicResp.Content) > 0 {
		commitMsg := strings.TrimSpace(anthropicResp.Content[0].Text)

		// Remove dry-run check and go straight to interactive mode
		fmt.Fprintf(os.Stderr, "\nSuggested commit message:\n------------------\n%s\n------------------\n", commitMsg)
		fmt.Fprintf(os.Stderr, "\nDo you want to (a)ccept, (e)dit, or (r)eject this message? ")

		for {
			choice := getInput("")
			switch choice {
			case "a", "accept":
				debug("Accepting commit message")
				if err := commitChanges(commitMsg); err != nil {
					fmt.Fprintln(os.Stderr, "Error committing changes:", err)
					os.Exit(1)
				}
				fmt.Fprintln(os.Stderr, "Changes committed successfully!")
				return

			case "e", "edit":
				debug("Editing commit message")
				edited, err := editMessage(commitMsg)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error editing message:", err)
					os.Exit(1)
				}
				if err := commitChanges(edited); err != nil {
					fmt.Fprintln(os.Stderr, "Error committing changes:", err)
					os.Exit(1)
				}
				fmt.Fprintln(os.Stderr, "Changes committed successfully!")
				return

			case "r", "reject":
				debug("Rejecting commit message")
				fmt.Fprintln(os.Stderr, "Commit message rejected. Exiting without committing.")
				os.Exit(0)

			default:
				fmt.Fprintf(os.Stderr, "Invalid choice. Please enter (a)ccept, (e)dit, or (r)eject: ")
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "Error: Empty response from API")
		os.Exit(1)
	}
}
