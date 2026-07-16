package server

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/rs/zerolog/log"
)

var demoUsers = []struct {
	Username string
	Password string
}{
	{"demo", "demo"},
	{"alice", "alice"},
	{"bob", "bob"},
	{"charlie", "charlie"},
	{"dave", "dave"},
}

var trackerNames = []string{
	"Project Alpha", "Project Beta", "Project Gamma", "Project Delta", "Project Epsilon",
	"Bug Tracker", "Feature Requests", "Sprint Velocity", "Release Pipeline", "Build Status",
	"Code Review Time", "PR Merge Latency", "Test Coverage", "Lint Warnings", "Security Scans",
	"API Response Time", "Error Rate", "Request Throughput", "Memory Usage", "CPU Load",
	"Disk I/O", "Network Latency", "Database Queries", "Cache Hit Rate", "Queue Depth",
	"Customer Tickets", "Ticket Resolution Time", "SLA Compliance", "Uptime Monitor", "Incident Count",
	"Daily Active Users", "New Registrations", "Churn Rate", "Revenue", "Cost Tracking",
	"Deployment Frequency", "Change Failure Rate", "MTTR", "MTBF", "Rollback Count",
	"Page Load Time", "First Paint", "LCP", "CLS", "FID",
	"Battery Usage", "Crash Rate", "ANR Rate", "App Start Time", "Screen FPS",
}

var paletteNames = []string{
	"default", "vintage", "dark", "infographic", "macarons", "essos", "halloween", "purple",
}

func trackerNamesForUser(idx, count int) []string {
	start := (idx * 10) % len(trackerNames)
	end := start + count
	if end > len(trackerNames) {
		end = len(trackerNames)
	}
	return trackerNames[start:end]
}

func (s *MoraServer) seedDemoData() error {
	log.Info().Msg("Seeding demo data")

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	now := time.Now()

	for i, du := range demoUsers {
		user, err := s.userStore.CreateUserWithPassword(du.Username, du.Password)
		if err != nil {
			return fmt.Errorf("create demo user %s: %w", du.Username, err)
		}
		log.Debug().Int64("user_id", user.ID).Str("username", du.Username).Msg("Created demo user")

		trackerCount := 8 + rng.Intn(3) // 8-10 per user
		names := trackerNamesForUser(i, trackerCount)

		for _, tname := range names {
			visibilities := []string{"public", "unlisted", "private"}
			visibility := visibilities[rng.Intn(len(visibilities))]

			palette := paletteNames[rng.Intn(len(paletteNames))]
			cc, _ := json.Marshal(map[string]string{"palette": palette})

			tracker, err := s.tracker.CreateTracker(tname, visibility, user.ID, "tracker", nil, string(cc))
			if err != nil {
				return fmt.Errorf("create demo tracker %s: %w", tname, err)
			}
			log.Debug().Int64("tracker_id", tracker.Id).Str("name", tname).Msg("Created demo tracker")

			seriesCount := 1 + rng.Intn(3) // 1-3 series per tracker
			seriesNames := []string{"count", "duration_ms", "score", "rate", "value"}
			rng.Shuffle(len(seriesNames), func(i, j int) {
				seriesNames[i], seriesNames[j] = seriesNames[j], seriesNames[i]
			})

			for si := 0; si < seriesCount; si++ {
				sname := seriesNames[si]
				dataType := "float"
				if sname == "count" {
					dataType = "integer"
				}

				series, err := s.tracker.CreateSeries(tracker.Id, sname, dataType)
				if err != nil {
					return fmt.Errorf("create demo series %s: %w", sname, err)
				}
				log.Debug().Int64("series_id", series.Id).Str("name", sname).Msg("Created demo series")

				valueCount := 10 + rng.Intn(11) // 10-20 values per series
				for vi := 0; vi < valueCount; vi++ {
					daysAgo := rng.Intn(90)
					hour := rng.Intn(24)
					min := rng.Intn(60)
					ts := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location()).AddDate(0, 0, -daysAgo)

					var val float64
					switch sname {
					case "count":
						val = float64(rng.Intn(1000))
					case "duration_ms":
						val = float64(100+rng.Intn(9900)) / 10.0
					case "score":
						val = float64(rng.Intn(100))
					case "rate":
						val = float64(rng.Intn(10000)) / 100.0
					default:
						val = float64(rng.Intn(10000)) / 100.0
					}

					if _, err := s.tracker.CreateValue(series.Id, ts, val); err != nil {
						return fmt.Errorf("create demo value: %w", err)
					}
				}
			}
		}
	}

	repos, err := s.repos.ListAll()
	if err == nil && len(repos) > 0 {
		adminID := int64(1)
		for _, repo := range repos {
			repoID := repo.Id
			_, err := s.tracker.CreateTracker(repo.Name+" coverage", "public", adminID, "coverage", &repoID, "")
			if err != nil {
				log.Warn().Err(err).Str("repo", repo.Name).Msg("Failed to create coverage tracker")
			}
		}
	}

	log.Info().Msg("Demo data seeded")
	return nil
}
