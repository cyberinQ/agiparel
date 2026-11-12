// Author: Sunil Jain, 2026-11-12.
// Embed the ten verified worker chapters and give Aggie a safe, read-only help source.
package workerhelp

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

//go:embed guide.json
var guideBytes []byte

type Chapter struct {
	Number   int      `json:"number"`
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle"`
	Path     string   `json:"path"`
	Goal     string   `json:"goal"`
	Have     string   `json:"have"`
	Steps    []string `json:"steps"`
	Done     string   `json:"done"`
	Stop     string   `json:"stop"`
	Mode     string   `json:"mode"`
}

type Manual struct {
	Version  string    `json:"version"`
	Scope    string    `json:"scope"`
	Chapters []Chapter `json:"chapters"`
}

var manual = loadManual()

func loadManual() Manual {
	var result Manual
	if err := json.Unmarshal(guideBytes, &result); err != nil || len(result.Chapters) != 10 {
		panic("worker guide failed validation")
	}
	for i, chapter := range result.Chapters {
		if chapter.Number != i+1 || chapter.Path == "" || len(chapter.Steps) != 6 {
			panic("worker guide chapter is incomplete")
		}
	}
	return result
}

func All() Manual { return manual }

func Get(number int) (Chapter, bool) {
	if number < 1 || number > len(manual.Chapters) {
		return Chapter{}, false
	}
	return manual.Chapters[number-1], true
}

func ForPath(path string) (Chapter, bool) {
	for _, ch := range manual.Chapters {
		if path == ch.Path {
			return ch, true
		}
	}
	return Chapter{}, false
}

var explicitChapter = regexp.MustCompile(`\b(?:chapter|stage|step)\s*0?([1-9]|10)\b`)

func isQuestionAboutInstructions(question string) bool {
	q := strings.ToLower(strings.TrimSpace(question))
	for _, phrase := range []string{
		"how do i", "how can i", "how to", "what should i", "where do i", "what do i click",
		"which button", "show me how", "help me", "walk me through", "instructions",
		"user manual", "worker manual", "guide me", "explain this screen", "steps for", "steps to",
		"how does this screen", "what does this button", "teach me", "help with", "full manual",
	} {
		if strings.Contains(q, phrase) {
			return true
		}
	}
	return false
}

func IsInstructionsQuestion(question string) bool { return isQuestionAboutInstructions(question) }

// Match returns an applicable chapter for a procedural request. The current page
// is only a hint; the worker's explicit chapter or task takes precedence.
func Match(question, page string) (Chapter, bool) {
	if !isQuestionAboutInstructions(question) {
		return Chapter{}, false
	}
	q := strings.ToLower(question)
	if m := explicitChapter.FindStringSubmatch(q); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		return Get(n)
	}
	// Specific task words take priority over a general reference to a buyer/order.
	cues := []struct {
		number int
		words  []string
	}{
		{10, []string{"upload", "document", "paper", "find a file", "download original"}},
		{8, []string{"pack", "carton", "dispatch", "shipping list"}},
		{7, []string{"quality", "check a piece", "defect", "repair", "finishing", "buyer tag"}},
		{6, []string{"sewing", "sew ", "stitch", "sewing job"}},
		{5, []string{"cutting", "cut a", "cut plan", "marker", "ply", "plies", "lay draft", "bundle"}},
		{4, []string{"fabric store", "fabric roll", "roll id", "shade", "roll check"}},
		{3, []string{"plan material", "material plan", "stock sheet", "cost sheet", "price sheet"}},
		{9, []string{"factory report", "progress report", "report page"}},
		{2, []string{"buyer order", "customer order", "purchase order", "po number", "tech pack"}},
		{1, []string{"factory setup", "machine count", "factory details"}},
	}
	for _, cue := range cues {
		for _, word := range cue.words {
			if strings.Contains(q, word) {
				return Get(cue.number)
			}
		}
	}
	return ForPath(page)
}

// Answer copies the reviewed instructions; it never generates a production fact.
func Answer(ch Chapter) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Chapter %02d — %s\nOpen: %s\nYour job: %s\nHave ready: %s\n", ch.Number, ch.Title, ch.Path, ch.Goal, ch.Have)
	for i, step := range ch.Steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, step)
	}
	fmt.Fprintf(&b, "DONE: %s\nSTOP IF: %s\n", ch.Done, ch.Stop)
	if ch.Mode == "practice" {
		b.WriteString("Remember: this practice screen is not proof of real production or approval.\n")
	}
	return b.String()
}

func Overview() string {
	var b strings.Builder
	b.WriteString("AGIPAREL has ten chapters. Pick a chapter below, then ask me how to do its steps.\n")
	for _, ch := range manual.Chapters {
		fmt.Fprintf(&b, "%02d. %s — %s\n", ch.Number, ch.Title, ch.Path)
	}
	b.WriteString("Saving a draft does not approve real stock, production, quality or shipping.")
	return b.String()
}

// FullText gives Aggie the complete reviewed worker manual, without inventing a step.
func FullText() string {
	var b strings.Builder
	for _, ch := range manual.Chapters {
		b.WriteString(Answer(ch))
		b.WriteString("\n")
	}
	return b.String()
}

func IsFullManualRequest(question string) bool {
	q := strings.ToLower(question)
	return strings.Contains(q, "full manual") || strings.Contains(q, "whole manual") ||
		strings.Contains(q, "all ten chapters") || strings.Contains(q, "all 10 chapters") ||
		strings.Contains(q, "entire manual")
}
