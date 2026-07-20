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

	type trackerEntry struct {
		id         int64
		ownerID    int64
		visibility string
	}
	var allTrackers []trackerEntry

	for i, du := range demoUsers {
		user, err := s.userStore.CreateUserWithPassword(du.Username, du.Password)
		if err != nil {
			return fmt.Errorf("create demo user %s: %w", du.Username, err)
		}
		log.Debug().Int64("user_id", user.ID).Str("username", du.Username).Msg("Created demo user")

		trackerCount := 8 + rng.Intn(3) // 8-10 per user
		names := trackerNamesForUser(i, trackerCount)

		for ti, tname := range names {
			visibilities := []string{"public", "private"}
			visibility := visibilities[rng.Intn(len(visibilities))]

			palette := paletteNames[rng.Intn(len(paletteNames))]
			cc := map[string]any{"palette": palette}

			// Every 3rd tracker gets a bar or mixed chart with multi-Y-axis
			if ti%3 == 0 {
				cc["y_axes"] = []map[string]any{
					{"id": 0, "label": "Count", "position": "left"},
					{"id": 1, "label": "Rate (%)", "position": "right", "min": 0, "max": 100},
				}
			}

			ccJSON, _ := json.Marshal(cc)

			tracker, err := s.tracker.CreateTracker(tname, visibility, user.ID, "tracker", nil, string(ccJSON))
			if err != nil {
				return fmt.Errorf("create demo tracker %s: %w", tname, err)
			}
			log.Debug().Int64("tracker_id", tracker.Id).Str("name", tname).Msg("Created demo tracker")

			allTrackers = append(allTrackers, trackerEntry{id: tracker.Id, ownerID: user.ID, visibility: visibility})

			seriesCount := 1 + rng.Intn(3) // 1-3 series per tracker
			seriesDefs := []struct {
				name     string
				dataType string
				barProb  int // probability (0-100) of being a bar chart
			}{
				{"count", "float", 40},
				{"duration_ms", "float", 10},
				{"score", "float", 30},
				{"rate", "float", 20},
				{"value", "float", 15},
			}
			rng.Shuffle(len(seriesDefs), func(i, j int) {
				seriesDefs[i], seriesDefs[j] = seriesDefs[j], seriesDefs[i]
			})

			hasYAxes := ti%3 == 0

			for si := 0; si < seriesCount; si++ {
				sd := seriesDefs[si]

				seriesConfig := map[string]any{}
				isBar := rng.Intn(100) < sd.barProb
				if isBar {
					seriesConfig["type"] = "bar"
				}
				if hasYAxes {
					// Assign "rate" series to right axis (id=1), others to left (id=0)
					if sd.name == "rate" || sd.name == "score" {
						seriesConfig["y_axis_index"] = 1
						seriesConfig["value_format"] = "%.1f%%"
					}
				}

				configJSON, _ := json.Marshal(seriesConfig)

				series, err := s.tracker.CreateSeries(tracker.Id, sd.name, sd.dataType, string(configJSON))
				if err != nil {
					return fmt.Errorf("create demo series %s: %w", sd.name, err)
				}
				log.Debug().Int64("series_id", series.Id).Str("name", sd.name).Msg("Created demo series")

				valueCount := 10 + rng.Intn(11) // 10-20 values per series
				for vi := 0; vi < valueCount; vi++ {
					daysAgo := rng.Intn(90)
					hour := rng.Intn(24)
					min := rng.Intn(60)
					ts := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location()).AddDate(0, 0, -daysAgo)

					var val float64
					switch sd.name {
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

	// Seed likes: each user randomly likes ~30% of other users' trackers
	for _, du := range demoUsers {
		user, err := s.userStore.FindByUsername(du.Username)
		if err != nil {
			continue
		}
		for _, t := range allTrackers {
			if t.ownerID == user.ID {
				continue // skip own trackers
			}
			if t.visibility == "private" {
				continue // skip private trackers
			}
			if rng.Intn(3) == 0 { // ~33%
				if err := s.tracker.Like(user.ID, t.id); err != nil {
					log.Warn().Err(err).Msg("Failed to seed like")
				}
			}
		}
	}

	log.Info().Msg("Demo data seeded")
	return nil
}
