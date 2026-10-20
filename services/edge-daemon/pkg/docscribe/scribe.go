package docscribe

import (
	"fmt"
	"strings"
)

type OperatorManual struct {
	StationID    string   `json:"station_id"`
	OperationName string  `json:"operation_name"`
	Language     string   `json:"language"`
	ReadingLevel string   `json:"reading_level"`
	Steps        []string `json:"steps"`
	AudioScript  string   `json:"audio_script"`
}

type DocScribeEngine struct {
	tenantID string
}

func NewDocScribeEngine(tenantID string) *DocScribeEngine {
	return &DocScribeEngine{tenantID: tenantID}
}

// GeneratePlainEnglishManual compiles 5th-grade instructions and audio prompts for shop floor operators.
func (s *DocScribeEngine) GeneratePlainEnglishManual(stationID, opName, language string) (OperatorManual, error) {
	lang := strings.ToUpper(language)
	if lang != "EN" && lang != "HI" && lang != "BN" && lang != "TA" {
		return OperatorManual{}, fmt.Errorf("ERR_UNSUPPORTED_LANGUAGE: Language %s not supported for audio prompt generation", language)
	}

	steps := []string{
		"1. Tap your NFC wristband on the station reader to log in.",
		"2. Scan the incoming bundle NFC tag.",
		"3. Complete the sewing operation following the 3D visual guide.",
		"4. Tap the large Green 'CYCLE DONE' button when finished.",
	}

	audioScript := "Please scan your bundle and press the green button when done."
	if lang == "HI" {
		audioScript = "कृपया अपना बंडल स्कैन करें और काम पूरा होने पर हरा बटन दबाएं।"
	} else if lang == "BN" {
		audioScript = "দয়া করে আপনার বান্ডিল স্ক্যান করুন এবং কাজ শেষ হলে সবুজ বোতাম টিপুন।"
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