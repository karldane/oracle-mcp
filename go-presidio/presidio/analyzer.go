package presidio

import "sort"

type AnalyzerEngine struct {
	registry     *RecognizerRegistry
	minScore     float64
	contextBoost bool
}

type AnalyzerConfig struct {
	Registry     *RecognizerRegistry
	MinScore     float64
	Entities     []EntityType
	ContextBoost bool
}

func NewAnalyzerEngine(cfg AnalyzerConfig) *AnalyzerEngine {
	a := &AnalyzerEngine{
		registry:     cfg.Registry,
		minScore:     0.5,
		contextBoost: true,
	}
	if cfg.MinScore > 0 {
		a.minScore = cfg.MinScore
	}
	if !cfg.ContextBoost {
		a.contextBoost = false
	}
	return a
}

func (a *AnalyzerEngine) AnalyseText(value string) []RecognizerResult {
	var results []RecognizerResult
	recognizers := a.registry.GetAll()
	for _, rec := range recognizers {
		results = append(results, rec.Analyse(value)...)
	}
	return a.filterAndBoost(results, value, "")
}

func (a *AnalyzerEngine) AnalyseColumn(columnName string, values []string) []RecognizerResult {
	var results []RecognizerResult

	if a.contextBoost {
		if entity, score := MatchColumnNameHint(columnName); entity != "" {
			results = append(results, RecognizerResult{
				EntityType: entity,
				Start:      -1,
				End:        -1,
				Score:      score,
				Recognizer: "column_name_hint",
			})
		}
	}

	if len(results) > 0 && results[0].Recognizer == "column_name_hint" && results[0].Score >= 0.85 {
		return a.filterResults(results)
	}

	for _, value := range values {
		recognizers := a.registry.GetAll()
		for _, rec := range recognizers {
			results = append(results, rec.Analyse(value)...)
		}
	}

	if a.contextBoost && len(values) > 0 {
		results = a.contextBoostFromValues(results, values)
	}

	return a.filterResults(results)
}

func (a *AnalyzerEngine) filterAndBoost(results []RecognizerResult, value, columnName string) []RecognizerResult {
	var filtered []RecognizerResult
	for _, r := range results {
		if r.Score >= a.minScore {
			filtered = append(filtered, r)
		}
	}
	return a.dedupeAndMerge(filtered)
}

func (a *AnalyzerEngine) filterResults(results []RecognizerResult) []RecognizerResult {
	var filtered []RecognizerResult
	for _, r := range results {
		if r.Score >= a.minScore {
			filtered = append(filtered, r)
		}
	}
	return a.dedupeAndMerge(filtered)
}

func (a *AnalyzerEngine) contextBoostFromValues(results []RecognizerResult, values []string) []RecognizerResult {
	if len(values) == 0 {
		return results
	}

	entityMatches := make(map[EntityType]int)
	for _, r := range results {
		entityMatches[r.EntityType]++
	}

	threshold := len(values) / 2
	for entity, count := range entityMatches {
		if count >= threshold {
			for i := range results {
				if results[i].EntityType == entity && results[i].Score < 0.80 {
					results[i].Score = 0.80
				}
			}
		}
	}

	return results
}

func (a *AnalyzerEngine) dedupeAndMerge(results []RecognizerResult) []RecognizerResult {
	if len(results) == 0 {
		return results
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Start != results[j].Start {
			return results[i].Start < results[j].Start
		}
		return results[i].End > results[j].End
	})

	var merged []RecognizerResult
	for _, r := range results {
		found := false
		for i := range merged {
			if merged[i].Start == r.Start && merged[i].End == r.End {
				if r.Score > merged[i].Score {
					merged[i] = r
				}
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, r)
		}
	}

	return merged
}
