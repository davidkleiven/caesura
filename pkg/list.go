package pkg

import (
	"log/slog"
	"slices"
	"strings"
)

const jaccardMatchThreshold = 0.01

type ItemsWithScore struct {
	Name  string
	Score float64
}

func lengthFromToken(token string, target int) int {
	if len(token) < target {
		return len(token)
	}
	return target
}

func FilterList(items []string, token string) []string {
	if token == "" {
		return items
	}

	tokenLength := lengthFromToken(token, 3)
	trigramsToken := Ngrams(strings.ToLower(token), tokenLength)
	itemWithScore := make([]ItemsWithScore, len(items))
	for i, item := range items {
		trigramsInstrument := Ngrams(strings.ToLower(item), tokenLength)
		itemWithScore[i].Name = item
		itemWithScore[i].Score = Jaccard(trigramsToken, trigramsInstrument)
	}

	// Filter low matches
	var i int
	for {
		if itemWithScore[i].Score < jaccardMatchThreshold {
			itemWithScore[i] = itemWithScore[len(itemWithScore)-1]
			itemWithScore = itemWithScore[:len(itemWithScore)-1]
		} else {
			i++
		}

		if i >= len(itemWithScore) {
			break
		}
	}

	slices.SortFunc(itemWithScore, func(a, b ItemsWithScore) int {
		if a.Score < b.Score {
			return 1 // Sort in descending order
		} else if a.Score > b.Score {
			return -1
		}
		return 0
	})

	slog.Info("Found instruments", "count", len(itemWithScore), "token", token)
	result := make([]string, len(itemWithScore))
	for i, item := range itemWithScore {
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
