package nerdgraph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var (
	USUrl = "https://api.newrelic.com/graphql"
	EUUrl = "https://api.eu.newrelic.com/graphql"
)

type Config struct {
	APIKey    string
	AccountID int
	Region    string
}

func GetURL(config Config) string {
	region := config.Region
	if region == "" {
		region = os.Getenv("NEW_RELIC_REGION")
	}
	if strings.ToUpper(region) == "EU" {
		return EUUrl
	}
	return USUrl
}

func GetAPIKey(config Config) (string, error) {
	key := config.APIKey
	if key == "" {
		key = os.Getenv("NEW_RELIC_API_KEY")
	}
	if key == "" {
		return "", fmt.Errorf("NEW_RELIC_API_KEY is not configured")
	}
	return key, nil
}

func GetAccountID(config Config) (int, error) {
	if config.AccountID != 0 {
		return config.AccountID, nil
	}
	idStr := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	if idStr != "" {
		id, err := strconv.Atoi(idStr)
		if err == nil {
			return id, nil
		}
	}
	return 0, fmt.Errorf("Account ID is required. Set NEW_RELIC_ACCOUNT_ID")
}

func NerdGraphQuery(config Config, query string, variables map[string]interface{}) (map[string]interface{}, error) {
	apiKey, err := GetAPIKey(config)
	if err != nil {
		return nil, err
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", GetURL(config), bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("API-Key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("NerdGraph request failed: %s", resp.Status)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
		var msgs []string
		for _, e := range errors {
			if em, ok := e.(map[string]interface{}); ok {
				if msg, ok := em["message"].(string); ok {
					msgs = append(msgs, msg)
				}
			}
		}
		return nil, fmt.Errorf("NerdGraph: %s", strings.Join(msgs, "; "))
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}
	return data, nil
}

const nrqlQueryTemplate = `
  query NrqlQuery($accounts: [Int!]!, $nrqlQuery: Nrql!) {
    actor {
      nrql(accounts: $accounts, query: $nrqlQuery) {
        results
        metadata {
          facets
          timeWindow { begin end }
        }
      }
    }
  }
`

func NRQLQuery(config Config, accountId int, nrql string) (map[string]interface{}, error) {
	variables := map[string]interface{}{
		"accounts":  []int{accountId},
		"nrqlQuery": nrql,
	}
	return NerdGraphQuery(config, nrqlQueryTemplate, variables)
}

func EntitySearchGql(domain string) string {
	if domain == "" {
		domain = "APM"
	}
	return fmt.Sprintf(`{
    actor {
      entitySearch(query: "domain = '%s' AND reporting = 'true'") {
        results {
          entities {
            guid
            name
            entityType
            alertSeverity
            reporting
            tags { key values }
            ... on ApmApplicationEntityOutline {
              apmSummary {
                responseTimeAverage
                throughput
                errorRate
                apdexScore
              }
            }
          }
        }
      }
    }
  }`, domain)
}

const DASHBOARD_SEARCH_GQL = `{
  actor {
    entitySearch(query: "domain = 'VIZ' AND type = 'DASHBOARD'") {
      results {
        entities {
          guid
          name
          accountId
          ... on DashboardEntityOutline {
            permalink
            updatedAt
            createdAt
          }
        }
      }
    }
  }
}`

const DASHBOARD_DETAIL_GQL = `
  query DashboardDetail($guid: EntityGuid!) {
    actor {
      entity(guid: $guid) {
        ... on DashboardEntity {
          name
          permissions
          accountId
          pages {
            guid
            name
            description
            widgets {
              id
              title
              visualization { id }
              layout { row column width height }
              rawConfiguration
            }
          }
        }
      }
    }
  }
`

func AlertIssuesGql(accountId int) string {
	return fmt.Sprintf(`{
    actor {
      account(id: %d) {
        aiIssues {
          issues(filter: { states: [ACTIVATED, CREATED] }) {
            issues {
              issueId
              title
              priority
              state
              activatedAt
              closedAt
              sources
              entityGuids
              entityNames
              conditionName
              totalIncidents
            }
          }
        }
      }
    }
  }`, accountId)
}
