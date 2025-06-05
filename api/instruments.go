package api

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/davidkleiven/caesura/utils"
)

var instruments = []string{
	"Accordion",
	"Acoustic Guitar",
	"Acoustic Piano",
	"Alto (Choir)",
	"Alto Saxophone",
	"Baritone Saxophone",
	"Baritone",
	"Bass (Choir)",
	"Bass Clarinet",
	"Bassoon",
	"Castanets",
	"Cello",
	"Clarinet",
	"Contrabassoon",
	"Cornet",
	"Double Bass",
	"Drum Kit",
	"English Horn",
	"Flugelhorn",
	"Flute",
	"French Horn",
	"Marimba",
	"Oboe",
	"Organ",
	"Percussion",
	"Piano",
	"Piccolo",
	"Soprano (Choir)",
	"Snare Drum",
	"Tambourine",
	"Tenor (Choir)",
	"Tenor Saxophone",
	"Triangle",
	"Trombone",
	"Trumpet",
	"Tuba",
	"Vibraphone",
	"Viola",
	"Violin",
	"Xylophone",
}

const jaccardMatchThreshold = 0.1

type InstrumentWithScore struct {
	Name  string
	Score float64
}

func Instruments(token string) []string {
	if token == "" {
		return instruments
	}
	trigramsToken := utils.Ngrams(strings.ToLower(token), 3)
	instrumentWithScore := make([]InstrumentWithScore, len(instruments))
	for i, instrument := range instruments {
		trigramsInstrument := utils.Ngrams(strings.ToLower(instrument), 3)
		instrumentWithScore[i].Name = instrument
		instrumentWithScore[i].Score = Jaccard(trigramsToken, trigramsInstrument)
	}

	// Filter low matches
	for i := 0; i < len(instrumentWithScore); i++ {
		if instrumentWithScore[i].Score < jaccardMatchThreshold {
			instrumentWithScore[i] = instrumentWithScore[len(instrumentWithScore)-1]
			instrumentWithScore = instrumentWithScore[:len(instrumentWithScore)-1]
		}
	}

	slices.SortFunc(instrumentWithScore, func(a, b InstrumentWithScore) int {
		if a.Score < b.Score {
			return 1 // Sort in descending order
		} else if a.Score > b.Score {
			return -1
		}
		return 0
	})

	slog.Info("Found instruments", "count", len(instrumentWithScore), "token", token)
	result := make([]string, len(instrumentWithScore))
	for i, item := range instrumentWithScore {
		result[i] = item.Name
	}
	return result
}

func Jaccard(a, b []string) float64 {
	setA := make(map[string]struct{}, len(a))
	for _, item := range a {
		setA[item] = struct{}{}
	}

	setB := make(map[string]struct{}, len(b))
	for _, item := range b {
		setB[item] = struct{}{}
	}

	intersectionCount := 0
	for item := range setA {
		if _, exists := setB[item]; exists {
			intersectionCount++
		}
	}

	unionCount := len(setA) + len(setB) - intersectionCount
	if unionCount == 0 {
		return 0.0 // Avoid division by zero
	}
	return float64(intersectionCount) / float64(unionCount)
}
