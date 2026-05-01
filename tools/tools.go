package tools

import (
	"context"
	"fmt"
	"strings"

	"new-relic-mcp/nerdgraph"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterTools(s *server.MCPServer) {
	// 1. nrql-query
	nrqlTool := mcp.NewTool("nrql-query",
		mcp.WithDescription("Execute a custom NRQL query against New Relic and visualize results with interactive charts. Use this for flexible, ad-hoc metric queries."),
		mcp.WithString("nrql", mcp.Required(), mcp.Description("NRQL query to execute. Must start with SELECT.")),
		mcp.WithNumber("accountId", mcp.Description("New Relic account ID (uses default if not provided)")),
	)
	s.AddTool(nrqlTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]interface{})
		if !ok {
			args = make(map[string]interface{})
		}

		nrql, _ := args["nrql"].(string)
		var accountId int
		if val, ok := args["accountId"].(float64); ok {
			accountId = int(val)
		}

		config := nerdgraph.Config{AccountID: accountId}
		acctId, err := nerdgraph.GetAccountID(config)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		config.AccountID = acctId

		data, err := nerdgraph.NRQLQuery(config, acctId, nrql)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}

		actor, _ := data["actor"].(map[string]interface{})
		nrqlRes, _ := actor["nrql"].(map[string]interface{})
		results, _ := nrqlRes["results"].([]interface{})

		if len(results) == 0 {
			return mcp.NewToolResultText("No results found."), nil
		}

		text := fmt.Sprintf("NRQL Results (%d rows):\n", len(results))
		for i, rowI := range results {
			if i >= 20 {
				text += "... (truncated to 20 rows)\n"
				break
			}
			if row, ok := rowI.(map[string]interface{}); ok {
				if len(row) == 1 {
					for k, v := range row {
						text += fmt.Sprintf("  %s: %s\n", k, FormatValue(v))
					}
				} else {
					var parts []string
					for k, v := range row {
						parts = append(parts, fmt.Sprintf("%s: %s", k, FormatValue(v)))
					}
					text += fmt.Sprintf("  { %s }\n", strings.Join(parts, ", "))
				}
			}
		}

		return mcp.NewToolResultText(text), nil
	})

	// 2. entity-health
	entityHealthTool := mcp.NewTool("entity-health",
		mcp.WithDescription("Get a high-level health summary of all APM or Browser applications in the account."),
		mcp.WithString("domain", mcp.Description("Entity domain to search: APM or BROWSER. Defaults to APM.")),
	)
	s.AddTool(entityHealthTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]interface{})
		if !ok {
			args = make(map[string]interface{})
		}

		domain := "APM"
		if val, ok := args["domain"].(string); ok && val != "" {
			domain = val
		}

		config := nerdgraph.Config{}
		entities, err := GetEntities(config, domain)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}

		text := fmt.Sprintf("Entity Health (%s) - %d entities found\n\n", domain, len(entities))
		var parts []string
		for _, eI := range entities {
			if e, ok := eI.(map[string]interface{}); ok {
				name := e["name"]
				alert := e["alertSeverity"]
				parts = append(parts, fmt.Sprintf("  %v [%v]", name, alert))
			}
		}
		text += strings.Join(parts, "\n")
		return mcp.NewToolResultText(text), nil
	})
}

func registerMoreTools(s *server.MCPServer) {
	// 3. alert-incidents
	alertTool := mcp.NewTool("alert-incidents",
		mcp.WithDescription("Get currently active AI-correlated alert issues across the account."),
		mcp.WithNumber("accountId", mcp.Description("Account ID")),
	)
	s.AddTool(alertTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]interface{})
		if !ok {
			args = make(map[string]interface{})
		}

		var accountId int
		if val, ok := args["accountId"].(float64); ok {
			accountId = int(val)
		}

		config := nerdgraph.Config{AccountID: accountId}
		acctId, err := nerdgraph.GetAccountID(config)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		config.AccountID = acctId

		data, err := nerdgraph.NerdGraphQuery(config, nerdgraph.AlertIssuesGql(acctId), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}

		actor, _ := data["actor"].(map[string]interface{})
		account, _ := actor["account"].(map[string]interface{})
		aiIssues, _ := account["aiIssues"].(map[string]interface{})
		issuesWrap, _ := aiIssues["issues"].(map[string]interface{})
		issues, _ := issuesWrap["issues"].([]interface{})

		if len(issues) == 0 {
			return mcp.NewToolResultText("No active alert issues found."), nil
		}

		text := fmt.Sprintf("Active Alerts (%d found):\n", len(issues))
		for _, isI := range issues {
			if is, ok := isI.(map[string]interface{}); ok {
				text += fmt.Sprintf("- [%s] %s (Incidents: %v)\n", is["priority"], is["title"], FormatValue(is["totalIncidents"]))
			}
		}

		return mcp.NewToolResultText(text), nil
	})

	// 6. log-search
	logSearchTool := mcp.NewTool("log-search",
		mcp.WithDescription("Search and analyze application logs from New Relic."),
		mcp.WithString("query", mcp.Description("Search term to filter log messages")),
		mcp.WithString("appName", mcp.Description("Filter logs to a specific application/service name")),
		mcp.WithString("level", mcp.Description("Log severity filter")),
		mcp.WithString("since", mcp.Description("Time range")),
		mcp.WithString("around", mcp.Description("ISO 8601 timestamp to center search on (± 5 minutes)")),
		mcp.WithBoolean("excludeAgentLogs", mcp.Description("Filter out New Relic agent internal log messages (default: true)")),
		mcp.WithNumber("accountId", mcp.Description("Account ID")),
	)
	s.AddTool(logSearchTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]interface{})
		if !ok {
			args = make(map[string]interface{})
		}

		query, _ := args["query"].(string)
		appName, _ := args["appName"].(string)
		level, _ := args["level"].(string)
		since, _ := args["since"].(string)
		if since == "" {
			since = "1 hour ago"
		}

		excludeAgentLogs := true
		if val, ok := args["excludeAgentLogs"].(bool); ok {
			excludeAgentLogs = val
		}

		var accountId int
		if val, ok := args["accountId"].(float64); ok {
			accountId = int(val)
		}

		config := nerdgraph.Config{AccountID: accountId}
		acctId, err := nerdgraph.GetAccountID(config)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		config.AccountID = acctId

		var baseClauses []string
		if level != "" && level != "ALL" {
			baseClauses = append(baseClauses, fmt.Sprintf("level = '%s'", level))
		}
		if query != "" {
			baseClauses = append(baseClauses, fmt.Sprintf("message LIKE '%%%s%%'", EscNrql(query)))
		}
		if excludeAgentLogs {
			baseClauses = append(baseClauses, `NOT (message LIKE '%rpm request%' OR message LIKE '%transaction ended%' OR message LIKE '%analytic_event_data%' OR message LIKE '%connect_reply%' OR message LIKE '%Reporting to%')`)
		}

		if appName != "" {
			resolved, _ := ResolveAppName(config, appName, "")
			if resolved != nil {
				app := EscNrql(resolved.Name)
				baseClauses = append(baseClauses, fmt.Sprintf("(entity.name = '%s' OR service.name = '%s' OR appName = '%s')", app, app, app))
			} else {
				baseClauses = append(baseClauses, fmt.Sprintf("appName LIKE '%%%s%%'", EscNrql(appName)))
			}
		}

		where := ""
		if len(baseClauses) > 0 {
			where = "WHERE " + strings.Join(baseClauses, " AND ")
		}

		timeClause := "SINCE " + since
		nrql := fmt.Sprintf("SELECT timestamp, level, message, entity.name as service FROM Log %s %s LIMIT 50", where, timeClause)

		data, err := nerdgraph.NRQLQuery(config, acctId, nrql)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}

		actor, _ := data["actor"].(map[string]interface{})
		nrqlRes, _ := actor["nrql"].(map[string]interface{})
		results, _ := nrqlRes["results"].([]interface{})

		if len(results) == 0 {
			return mcp.NewToolResultText("No logs found."), nil
		}

		text := fmt.Sprintf("Logs (%d results):\n\n", len(results))
		for _, rI := range results {
			if r, ok := rI.(map[string]interface{}); ok {
				text += fmt.Sprintf("[%s] %s | %s: %s\n", FormatValue(r["timestamp"]), FormatValue(r["service"]), FormatValue(r["level"]), FormatValue(r["message"]))
			}
		}

		return mcp.NewToolResultText(text), nil
	})
}

func registerEvenMoreTools(s *server.MCPServer) {
	// 8. describe-event
	describeTool := mcp.NewTool("describe-event",
		mcp.WithDescription("Discover the schema of a New Relic event type."),
		mcp.WithString("eventType", mcp.Required(), mcp.Description("The NRQL event type to describe")),
		mcp.WithString("appName", mcp.Description("Optional: filter to a specific app")),
		mcp.WithNumber("accountId", mcp.Description("Account ID")),
	)
	s.AddTool(describeTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]interface{})
		if !ok {
			args = make(map[string]interface{})
		}

		eventType, _ := args["eventType"].(string)
		appName, _ := args["appName"].(string)

		var accountId int
		if val, ok := args["accountId"].(float64); ok {
			accountId = int(val)
		}

		config := nerdgraph.Config{AccountID: accountId}
		acctId, err := nerdgraph.GetAccountID(config)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		config.AccountID = acctId

		where := ""
		if appName != "" {
			resolved, _ := ResolveAppName(config, appName, "")
			if resolved != nil {
				where = fmt.Sprintf("WHERE appName = '%s' OR entity.name = '%s'", EscNrql(resolved.Name), EscNrql(resolved.Name))
			}
		}

		nrql := fmt.Sprintf("SELECT keyset() FROM %s %s SINCE 1 day ago", EscNrql(eventType), where)
		data, err := nerdgraph.NRQLQuery(config, acctId, nrql)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}

		actor, _ := data["actor"].(map[string]interface{})
		nrqlRes, _ := actor["nrql"].(map[string]interface{})
		results, _ := nrqlRes["results"].([]interface{})

		if len(results) == 0 {
			return mcp.NewToolResultText("No attributes found for event type: " + eventType), nil
		}

		var keys []string
		for _, rI := range results {
			if r, ok := rI.(map[string]interface{}); ok {
				if k, ok := r["stringKeys"].([]interface{}); ok {
					for _, ki := range k { keys = append(keys, fmt.Sprintf("%v (string)", ki)) }
				}
				if k, ok := r["numericKeys"].([]interface{}); ok {
					for _, ki := range k { keys = append(keys, fmt.Sprintf("%v (numeric)", ki)) }
				}
				if k, ok := r["booleanKeys"].([]interface{}); ok {
					for _, ki := range k { keys = append(keys, fmt.Sprintf("%v (boolean)", ki)) }
				}
			}
		}

		text := fmt.Sprintf("Schema for %s:\n\n", eventType)
		text += strings.Join(keys, "\n")
		return mcp.NewToolResultText(text), nil
	})

	// 10. error-inbox
	errorInboxTool := mcp.NewTool("error-inbox",
		mcp.WithDescription("View grouped error fingerprints for an application."),
		mcp.WithString("appName", mcp.Required(), mcp.Description("Exact APM or Browser application name")),
		mcp.WithString("since", mcp.Description("Time range: '1 hour ago', '1 day ago'")),
		mcp.WithNumber("accountId", mcp.Description("Account ID")),
	)
	s.AddTool(errorInboxTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]interface{})
		if !ok {
			args = make(map[string]interface{})
		}

		appName, _ := args["appName"].(string)
		since, _ := args["since"].(string)
		if since == "" { since = "1 day ago" }

		var accountId int
		if val, ok := args["accountId"].(float64); ok {
			accountId = int(val)
		}

		config := nerdgraph.Config{AccountID: accountId}
		acctId, err := nerdgraph.GetAccountID(config)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		config.AccountID = acctId

		resolved, err := ResolveAppName(config, appName, "")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		domainClause := "TransactionError"
		if resolved.Domain == "BROWSER" {
			domainClause = "JavaScriptError"
		}

		nrql := fmt.Sprintf("SELECT count(*) as 'occurrences', uniqueCount(error.group.userImpact) as 'users' FROM %s WHERE appName = '%s' FACET error.message, error.class SINCE %s LIMIT 10", domainClause, EscNrql(resolved.Name), since)
		data, err := nerdgraph.NRQLQuery(config, acctId, nrql)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}

		actor, _ := data["actor"].(map[string]interface{})
		nrqlRes, _ := actor["nrql"].(map[string]interface{})
		results, _ := nrqlRes["results"].([]interface{})

		if len(results) == 0 {
			return mcp.NewToolResultText("No errors found."), nil
		}

		text := fmt.Sprintf("Top Errors for %s:\n\n", resolved.Name)
		for _, rI := range results {
			if r, ok := rI.(map[string]interface{}); ok {
				text += fmt.Sprintf("- [%s] %s\n  Occurrences: %v, Impacted Users: %v\n", FormatFacet(r["facet"]), FormatFacet(r["facet"]), FormatValue(r["occurrences"]), FormatValue(r["users"]))
			}
		}

		return mcp.NewToolResultText(text), nil
	})
}

func registerEvenMoreMoreTools(s *server.MCPServer) {
	// 15. dashboard
	dashboardTool := mcp.NewTool("dashboard",
		mcp.WithDescription("List New Relic dashboards or render a specific dashboard inline."),
		mcp.WithString("dashboardGuid", mcp.Description("Dashboard entity GUID. Omit to list all dashboards.")),
		mcp.WithNumber("accountId", mcp.Description("Account ID")),
	)
	s.AddTool(dashboardTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]interface{})
		if !ok {
			args = make(map[string]interface{})
		}

		dashboardGuid, _ := args["dashboardGuid"].(string)

		var accountId int
		if val, ok := args["accountId"].(float64); ok {
			accountId = int(val)
		}

		config := nerdgraph.Config{AccountID: accountId}
		_, err := nerdgraph.GetAccountID(config)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if dashboardGuid == "" {
			data, err := nerdgraph.NerdGraphQuery(config, nerdgraph.DASHBOARD_SEARCH_GQL, nil)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
			}
			actor, _ := data["actor"].(map[string]interface{})
			entitySearch, _ := actor["entitySearch"].(map[string]interface{})
			results, _ := entitySearch["results"].(map[string]interface{})
			dashboards, _ := results["entities"].([]interface{})

			text := fmt.Sprintf("Dashboards (%d found)\n\n", len(dashboards))
			for _, dI := range dashboards {
				if d, ok := dI.(map[string]interface{}); ok {
					text += fmt.Sprintf("- %s (GUID: %s)\n", d["name"], d["guid"])
				}
			}
			return mcp.NewToolResultText(text), nil
		} else {
			variables := map[string]interface{}{"guid": dashboardGuid}
			data, err := nerdgraph.NerdGraphQuery(config, nerdgraph.DASHBOARD_DETAIL_GQL, variables)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
			}
			actor, _ := data["actor"].(map[string]interface{})
			entity, _ := actor["entity"].(map[string]interface{})

			text := fmt.Sprintf("Dashboard: %v\n\n", entity["name"])
			if pages, ok := entity["pages"].([]interface{}); ok {
				for _, pI := range pages {
					if p, ok := pI.(map[string]interface{}); ok {
						if widgets, ok := p["widgets"].([]interface{}); ok {
							for _, wI := range widgets {
								if w, ok := wI.(map[string]interface{}); ok {
									text += fmt.Sprintf("Widget: %v\n", w["title"])
								}
							}
						}
					}
				}
			}
			return mcp.NewToolResultText(text), nil
		}
	})
}

func init() {
}
