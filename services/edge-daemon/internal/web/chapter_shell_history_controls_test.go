// Author: Sunil Jain, 2026-10-28
// Chapter arrows link to adjacent Home chapters, including after direct page entry.
// There are no dead browser-history arrows and no fake endpoint destinations.
package web

import (
	"regexp"
	"strings"
	"testing"
)

func TestSharedChapterSequenceNavigation(t *testing.T) {
	raw, err := staticFS.ReadFile("static/workspace_nav.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(raw)
	home, err := staticFS.ReadFile("static/portal.html")
	if err != nil {
		t.Fatal(err)
	}

	// The single navigation sequence must exactly follow the ten Home tiles.
	chapterPattern := regexp.MustCompile(`\{href:'(/[^']+)', phase:'[^']+', name:'[^']+'\}`)
	matches := chapterPattern.FindAllStringSubmatch(js, -1)
	tilePattern := regexp.MustCompile(`class="phase phase-\d\d" href="([^"]+)"`)
	tiles := tilePattern.FindAllStringSubmatch(string(home), -1)
	if len(matches) != 10 || len(tiles) != 10 {
		t.Fatalf("expected ten chapters in both JS and Home, got %d and %d", len(matches), len(tiles))
	}
	for i := range matches {
		if matches[i][1] != tiles[i][1] {
			t.Errorf("chapter %02d: navigation %q differs from Home %q", i+1, matches[i][1], tiles[i][1])
		}
	}
	for _, required := range []string{
		"currentChapterIndex = chapters.findIndex", "currentChapterIndex > 0",
		"currentChapterIndex < chapters.length - 1", "currentChapterIndex - 1",
		"currentChapterIndex + 1", "back.href = previousChapter.href",
		"forward.href = nextChapter.href", "home.href = '/'",
		"aria-label', 'Previous chapter: '", "aria-label', 'Next chapter: '",
		"ag-chapter-history-group", "home.innerHTML = '<svg",
		"actions.append(historyGroup)",
	} {
		if !strings.Contains(js, required) {
			t.Errorf("missing chapter navigation %q", required)
		}
	}
	for _, forbidden := range []string{"history.back()", "history.forward()", "history.length", "addEventListener('click'"} {
		if strings.Contains(js, forbidden) {
			t.Errorf("navigation must be direct links, found %q", forbidden)
		}
	}
	css, err := staticFS.ReadFile("static/workspace_nav.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{".ag-chapter-history-group", ".ag-chapter-history", ".ag-chapter-home svg", "height:42px"} {
		if !strings.Contains(string(css), required) {
			t.Errorf("navigation style missing %q", required)
		}
	}
}
