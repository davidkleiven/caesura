package pkg

func Ngrams(text string, n int) []string {
	if n <= 0 || len(text) < n {
		return []string{text}
	}

	ngrams := make([]string, 0, len(text)-n+1)
	for i := 0; i <= len(text)-n; i++ {
		ngrams = append(ngrams, text[i:i+n])
	}
	return ngrams
}
