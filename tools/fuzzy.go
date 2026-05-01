package tools

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ikozinov/new-relic-mcp/nerdgraph"
)

type ResolvedEntity struct {
	Name   string
	GUID   string
	Domain string
	Match  string
}

var (
	entityCache = make(map[string]struct {
		entities []interface{}
		ts       time.Time
	})
	cacheMutex sync.Mutex
	cacheTTL   = 5 * time.Minute
)

func GetEntities(config nerdgraph.Config, domain string) ([]interface{}, error) {
	if domain == "" {
		domain = "APM"
	}
	cacheMutex.Lock()
	cached, ok := entityCache[domain]
	if ok && time.Since(cached.ts) < cacheTTL {
		cacheMutex.Unlock()
		return cached.entities, nil
	}
	cacheMutex.Unlock()

	data, err := nerdgraph.NerdGraphQuery(config, nerdgraph.EntitySearchGql(domain), nil)
	if err != nil {
		if ok {
			return cached.entities, nil
		}
		return nil, err
	}

	actor, _ := data["actor"].(map[string]interface{})
	entitySearch, _ := actor["entitySearch"].(map[string]interface{})
	results, _ := entitySearch["results"].(map[string]interface{})
	entities, _ := results["entities"].([]interface{})
	if entities == nil {
		entities = []interface{}{}
	}

	cacheMutex.Lock()
	entityCache[domain] = struct {
		entities []interface{}
		ts       time.Time
	}{entities, time.Now()}
	cacheMutex.Unlock()

	return entities, nil
}

func ResolveEntity(config nerdgraph.Config, input string, domain string) (*ResolvedEntity, error) {
	entities, err := GetEntities(config, domain)
	if err != nil {
		return nil, err
	}
	if input == "" {
		return nil, nil
	}

	reg := regexp.MustCompile(`[\s_-]+`)
	lower := reg.ReplaceAllString(strings.ToLower(input), "")

	var named []map[string]interface{}
	for _, e := range entities {
		if em, ok := e.(map[string]interface{}); ok {
			if name, ok := em["name"].(string); ok && name != "" {
				named = append(named, em)
			}
		}
	}

	// 1. Exact
	for _, e := range named {
		if e["name"].(string) == input {
			return &ResolvedEntity{Name: e["name"].(string), GUID: e["guid"].(string), Domain: domain, Match: "exact"}, nil
		}
	}

	// 2. Case-insensitive
	for _, e := range named {
		if strings.ToLower(e["name"].(string)) == strings.ToLower(input) {
			return &ResolvedEntity{Name: e["name"].(string), GUID: e["guid"].(string), Domain: domain, Match: "case-insensitive"}, nil
		}
	}

	// 3. Fuzzy
	for _, e := range named {
		if reg.ReplaceAllString(strings.ToLower(e["name"].(string)), "") == lower {
			return &ResolvedEntity{Name: e["name"].(string), GUID: e["guid"].(string), Domain: domain, Match: "fuzzy"}, nil
		}
	}

	// 4. Substring
	for _, e := range named {
		enameLower := strings.ToLower(e["name"].(string))
		inLower := strings.ToLower(input)
		if strings.Contains(enameLower, inLower) || strings.Contains(inLower, enameLower) {
			return &ResolvedEntity{Name: e["name"].(string), GUID: e["guid"].(string), Domain: domain, Match: "fuzzy"}, nil
		}
	}

	return nil, nil
}

func ResolveAppName(config nerdgraph.Config, input string, preferredDomain string) (*ResolvedEntity, error) {
	domains := []string{"APM", "BROWSER"}
	if preferredDomain != "" {
		domains = []string{preferredDomain}
	}
	for _, d := range domains {
		res, err := ResolveEntity(config, input, d)
		if err != nil {
			continue
		}
		if res != nil {
			return res, nil
		}
	}
	return nil, fmt.Errorf("Entity \"%s\" not found. Similar names: %s", input, SuggestEntities(config, input))
}

func SuggestEntities(config nerdgraph.Config, input string) string {
	var all []map[string]interface{}
	entitiesAPM, _ := GetEntities(config, "APM")
	for _, e := range entitiesAPM {
		if em, ok := e.(map[string]interface{}); ok {
			all = append(all, em)
		}
	}
	entitiesB, _ := GetEntities(config, "BROWSER")
	for _, e := range entitiesB {
		if em, ok := e.(map[string]interface{}); ok {
			all = append(all, em)
		}
	}

	lower := strings.ToLower(input)
	reg := regexp.MustCompile(`[\s_-]+`)
	inputWords := reg.Split(lower, -1)

	type Scored struct {
		Name  string
		Score int
	}
	var scored []Scored

	for _, e := range all {
		name, ok := e["name"].(string)
		if !ok || name == "" {
			continue
		}
		nlower := strings.ToLower(name)
		score := 0
		if strings.Contains(nlower, lower) || strings.Contains(lower, nlower) {
			score += 3
		}
		nameWords := reg.Split(nlower, -1)
		for _, w := range inputWords {
			for _, nw := range nameWords {
				if strings.Contains(nw, w) || strings.Contains(w, nw) {
					score += 1
				}
			}
		}
		if score > 0 {
			scored = append(scored, Scored{Name: name, Score: score})
		}
	}

	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].Score > scored[i].Score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}
	if len(scored) > 5 {
		scored = scored[:5]
	}

	if len(scored) == 0 {
		return "(no similar names found — use entity-health to list all)"
	}
	var s []string
	for _, sc := range scored {
		s = append(s, "\""+sc.Name+"\"")
	}
	return strings.Join(s, ", ")
}
