package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type GitHubEvent struct {
	Type      string       `json:"type"`
	Repo      GitHubRepo   `json:"repo"`
	Payload   EventPayload `json:"payload"`
	CreatedAt string       `json:"created_at"`
}

type GitHubRepo struct {
	Name string `json:"name"`
}

type EventPayload struct {
	Head string `json:"head"`
}

type HackatimeStats struct {
	Data UserStats `json:"data"`
}

type UserStats struct {
	TotalSeconds              int64          `json:"total_seconds"`
	DailyAverage              int64          `json:"daily_average"`
	HumanReadableTotal        string         `json:"human_readable_total"`
	HumanReadableDailyAverage string         `json:"human_readable_daily_average"`
	Languages                 []LanguageStat `json:"languages"`
}

type LanguageStat struct {
	Name         string  `json:"name"`
	TotalSeconds int64   `json:"total_seconds"`
	Text         string  `json:"text"`
	Hours        int64   `json:"hours"`
	Minutes      int64   `json:"minutes"`
	Percent      float64 `json:"percent"`
	Digital      string  `json:"digital"`
}

type WakatimeParams struct {
	start_date string
	end_date   string
}

type ReadmeParams struct {
	LastRepo      string
	LastCommit    string
	HoursWorked   string
	HackatimeData string
	UpdatedDate   string
	UpdatedTime   string
}

func main() {

	var params = ReadmeParams{}

	// check if the first argument is "local" to load the .env file

	if len(os.Args) > 1 && os.Args[1] == "local" {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}
	}
	// Implementation for generating README goes here

	var slackID = os.Getenv("SLACK_ID")
	var githubUsername = os.Getenv("GITHUB_USERNAME")

	// fetch github data

	respGitHub, err := http.Get("https://api.github.com/users/" + githubUsername + "/events/public")
	if err != nil {
		log.Fatal(err)
	}
	defer respGitHub.Body.Close()

	var events []GitHubEvent
	if err := json.NewDecoder(respGitHub.Body).Decode(&events); err != nil {
		log.Fatal(err)
	}

	for _, event := range events {
		if event.Type == "PushEvent" {
			params.LastRepo = event.Repo.Name
			params.LastCommit = fmt.Sprintf("[%s](https://github.com/%s/commit/%s)", event.Payload.Head[:7], event.Repo.Name, event.Payload.Head)
			break
		}
	}

	// fetch hackatime data

	resp, err := http.Get("https://hackatime.hackclub.com/api/v1/users/" + slackID + "/stats")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var allTimeStats HackatimeStats
	if err := json.NewDecoder(resp.Body).Decode(&allTimeStats); err != nil {
		panic(err)
	}

	var yesterday = time.Now().AddDate(0, 0, -1)
	// generate start/end time being 00:01 yesterday UTC+1 to 23:59 yesterday UTC+1
	// to get daily stats

	var startDate = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 1, 0, 0, time.UTC).Add(1 * time.Hour)
	var endDate = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 0, 0, time.UTC)

	url := "https://hackatime.hackclub.com/api/v1/users/" + slackID +
		"/stats?start_date=%22" + startDate.Format(time.RFC3339) +
		"%22&end_date=%22" + endDate.Format(time.RFC3339) + "%22"

	println("Fetching daily stats from URL: ", url)
	respDaily, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer respDaily.Body.Close()

	var dailyStats HackatimeStats
	if err := json.NewDecoder(respDaily.Body).Decode(&dailyStats); err != nil {
		panic(err)
	}

	// set params for hackatime

	params.HoursWorked = "4.5 hours"
	params.HackatimeData = generateLanguageBars(allTimeStats.Data.Languages, 10)

	// set date and time

	now := time.Now()
	params.UpdatedDate = now.Format("02 Jan 2006")
	params.UpdatedTime = now.Format("15:04 MST")

	// generate README.md from template

	tmpl, err := template.ParseFiles("README.template.md")
	if err != nil {
		log.Fatalf("Error parsing template: %v. Check if 'README.template.md' exists.", err)
	}

	// Create the output file
	f, err := os.Create("../README.md")
	if err != nil {
		log.Fatal("Error creating README.md: ", err)
	}
	defer f.Close()

	// Execute the template
	if err := tmpl.Execute(f, params); err != nil {
		log.Fatal("Error executing template: ", err)
	}
}

func generateLanguageBars(langs []LanguageStat, top int) string {
	if len(langs) < top {
		top = len(langs)
	}
	if top == 0 {
		return ""
	}

	md := ""
	barLength := 20

	maxHours := langs[0].Hours
	if maxHours == 0 {
		maxHours = 1
	}

	for i := 0; i < top; i++ {
		lang := langs[i]
		var filled int
		if i == 0 {
			filled = barLength
		} else {
			filled = int(float64(lang.Hours)/float64(maxHours)*float64(barLength) + 0.5) // relative to first
		}
		if filled > barLength {
			filled = barLength
		}
		empty := barLength - filled
		bar := fmt.Sprintf("[%s%s] %dh \\", strings.Repeat("█", filled), strings.Repeat("░", empty), lang.Hours)
		md += fmt.Sprintf("-> %s %s\n", lang.Name, bar)
	}

	return md
}
