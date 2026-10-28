package docscribe

import (
	"fmt"
	"strings"
)

type OperatorManual struct {
	StationID     string   `json:"station_id"`
	OperationName string   `json:"operation_name"`
	Language      string   `json:"language"`
	ReadingLevel  string   `json:"reading_level"`
	Steps         []string `json:"steps"`
	AudioScript   string   `json:"audio_script"`
}

type DocScribeEngine struct {
	tenantID string
}

func NewDocScribeEngine(tenantID string) *DocScribeEngine {
	return &DocScribeEngine{tenantID: tenantID}
}

func (s *DocScribeEngine) GeneratePlainEnglishManual(stationID, opName, language string) (OperatorManual, error) {
	lang := strings.ToUpper(language)
	if lang != "EN" && lang != "HI" && lang != "BN" && lang != "TA" {
		return OperatorManual{}, fmt.Errorf("ERR_UNSUPPORTED_LANGUAGE: Language %s not supported", language)
	}

	steps := []string{
		"1. Tap your NFC badge on the station reader to verify your shift.",
		fmt.Sprintf("2. Scan bundle tag and verify part labels for: %s.", opName),
		fmt.Sprintf("3. Execute operation '%s' matching seam margin and stitch pitch.", opName),
		"4. Inspect piece for tension/oil defects, then tap the Green button to advance bundle.",
	}

	audioScript := fmt.Sprintf("Station %s ready for %s. Scan your bundle to begin.", stationID, opName)

	switch lang {
	case "HI":
		audioScript = "\u0915\u0943\u092a\u092f\u093e \u0905\u092a\u0928\u093e \u092c\u0902\u0921\u0932 \u0938\u094d\u0915\u0948\u0928 \u0915\u0930\u0947\u0902 \u0914\u0930 \u0915\u093e\u092e \u092a\u0942\u0930\u093e \u0939\u094b\u0928\u0947 \u092a\u0930 \u0939\u0930\u093e \u092c\u091f\u0928 \u0926\u092c\u093e\u090f\u0902\u0964"
	case "BN":
		audioScript = "\u0985\u09a8\u09c1\u0997\u09cd\u09b0\u09b9 \u0995\u09b0\u09c7 \u0986\u09aa\u09a8\u09be\u09b0 \u09ac\u09be\u09a8\u09cd\u09a1\u09bf\u09b2 \u09b8\u09cd\u0995\u09cd\u09af\u09be\u09a8 \u0995\u09b0\u09c1\u09a8 \u098f\u09ac\u0982 \u0995\u09be\u099c \u09b6\u09c7\u09b7 \u09b9\u09b2\u09c7 \u09b8\u09ac\u09c1\u099c \u09ac\u09cb\u09a4\u09be\u09ae\u099f\u09bf \u099f\u09bf\u09aa\u09c1\u09a8\u0964"
	case "TA":
		audioScript = "\u0ba4\u0baf\u0bb5\u0bc1\u0b9a\u0bc6\u0baf\u0bcd\u0ba4\u0bc1 \u0b89\u0b99\u0bcd\u0b95\u0bb3\u0bcd \u0baa\u0ba3\u0bcd\u0b9f\u0bbf\u0bb2\u0bc8 \u0bb8\u0bcd\u0b95\u0bc7\u0ba9\u0bcd \u0b9a\u0bc6\u0baf\u0bcd\u0ba4\u0bc1, \u0bae\u0bc1\u0b9f\u0bbf\u0ba8\u0bcd\u0ba4\u0ba4\u0bc1\u0bae\u0bcd \u0baa\u0b9a\u0bcd\u0b9a\u0bc8 \u0baa\u0b9f\u0bcd\u0b9f\u0ba9\u0bc8 \u0b85\u0bb4\u0bc1\u0ba4\u0bcd\u0ba4\u0bb5\u0bc1\u0bae\u0bcd."
	}

	return OperatorManual{
		StationID:     stationID,
		OperationName: opName,
		Language:      lang,
		ReadingLevel:  "5TH_GRADE",
		Steps:         steps,
		AudioScript:   audioScript,
	}, nil
}
