package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

func fmtSonarrHealth() []string {
	issues, err := getSonarrHealth()
	if err != nil {
		return []string{fmt.Sprintf("Error getting Sonarr health: %v", err)}
	}

	var messages []string

	for _, issue := range issues {
		messages = append(messages, issue.Message)
	}

	return messages
}

type sonarrHealthIssue struct {
	Type    string `json:type`
	Message string `json:message`
	WikiURL string `json:wikiUrl`
}

func getSonarrHealth() ([]sonarrHealthIssue, error) {
	url, err := url.Parse("http://localhost/sonarr/api/health")
	if err != nil {
		panic(err)
	}
	q := url.Query()
	q.Set("apikey", config.secret.Sonarr.ApiKey)
	url.RawQuery = q.Encode()

	resp, err := http.Get(url.String())
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch %v: %v", url, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("Fetching %v: %s", url, resp.Status)
	}

	defer resp.Body.Close()

	var healthItems []sonarrHealthIssue
	json.NewDecoder(resp.Body).Decode(&healthItems)

	return healthItems, nil
}
