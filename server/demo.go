package server

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
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

var trackerDescriptions = []string{
	"Tracks team velocity and sprint progress",
	"Monitors application error rates and trends",
	"Measures API response times and latency",
	"Tracks test coverage metrics across repositories",
	"Monitors build pipeline status and duration",
	"Tracks deployment frequency and success rate",
	"Measures code review turnaround time",
	"Monitors infrastructure resource utilization",
	"Tracks customer support ticket volume",
	"Measures database query performance",
	"Tracks user engagement and retention metrics",
	"Monitors security scan findings and resolution",
	"Tracks memory and CPU usage over time",
	"Measures network latency and throughput",
	"Tracks feature adoption and usage metrics",
	"Monitors uptime and service level objectives",
	"Tracks release pipeline stages and duration",
	"Measures cost tracking and budget utilization",
	"Tracks incident frequency and resolution time",
	"Monitors page load performance metrics",
}

var demoBodies = []string{
	"## Overview\n\nThis tracker captures the metric over time.\n\n- Updated weekly\n- Source: automated pipeline\n\nSee [docs](https://example.com) for details.\n",
	"## How to read this chart\n\n1. Hover points for exact values\n2. Use the time range selector to zoom\n\n```\nslope = (y2 - y1) / (x2 - x1)\n```\n",
	"## Notes\n\n> A rising trend is expected during release weeks.\n\n**Watch for** sudden drops in the value.\n",
	"## Goals\n\n- [x] Baseline established\n- [ ] Reach target next quarter\n\n| Quarter | Target |\n|---------|--------|\n| Q1      | 80%    |\n| Q2      | 90%    |\n",
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
		ownerName  string
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
			xAxisType := "date"
			if rng.Intn(2) == 0 {
				xAxisType = "datetime"
			}
		cc := map[string]any{"palette": palette, "x_axis_type": xAxisType}

		// ~25% of trackers get area fill disabled
		if rng.Intn(4) == 0 {
			cc["area"] = false
		}

		// ~50% of trackers get symbols hidden, rest get symbols shown
		if rng.Intn(2) == 0 {
			cc["show_symbols"] = false
		} else {
			cc["show_symbols"] = true
		}

		// ~30% of trackers get slider hidden to mix it up
		if rng.Intn(10) < 3 {
			cc["show_slider"] = false
		}

		// Every 3rd tracker gets a bar or mixed chart with multi-Y-axis
			if ti%3 == 0 {
				cc["y_axes"] = []map[string]any{
					{"id": 0, "label": "Count", "position": "left"},
					{"id": 1, "label": "Rate (%)", "position": "right", "min": 0, "max": 100},
				}
			}

			ccJSON, _ := json.Marshal(cc)

			desc := trackerDescriptions[rng.Intn(len(trackerDescriptions))]

			// ~30% of trackers get a markdown body to showcase the feature
			var body string
			if rng.Intn(10) < 3 {
				body = demoBodies[rng.Intn(len(demoBodies))]
			}

			tracker, err := s.tracker.CreateTracker(tname, desc, body, visibility, user.ID, "tracker", string(ccJSON))
			if err != nil {
				return fmt.Errorf("create demo tracker %s: %w", tname, err)
			}
			log.Info().Int64("tracker_id", tracker.Id).Str("name", tname).
				Str("visibility", visibility).Str("owner", du.Username).
				Msg("Demo tracker created")

			allTrackers = append(allTrackers, trackerEntry{id: tracker.Id, ownerID: user.ID, ownerName: du.Username, visibility: visibility})

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

			seriesSeed := int(series.Id*7 + tracker.Id*13)
			valueCount := 10 + rng.Intn(11) // 10-20 values per series
			var daysList []int
		if xAxisType == "date" {
			daysList = rng.Perm(90)[:valueCount]
			sort.Ints(daysList)
		}
			for vi := 0; vi < valueCount; vi++ {
				var daysAgo int
				if xAxisType == "date" {
					daysAgo = daysList[vi]
				} else {
					daysAgo = rng.Intn(90)
				}
				hour := 0
				min := 0
				if xAxisType == "datetime" {
					hour = rng.Intn(24)
					min = rng.Intn(60)
				}
				ts := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location()).AddDate(0, 0, -daysAgo)

					var val float64
					vi64 := float64(vi)
					seed64 := float64(seriesSeed)
					switch sd.name {
					case "count":
						val = math.Sin(vi64*0.1+seed64)*300 +
							math.Sin(vi64*0.03+seed64*2)*200 +
							(rng.Float64()-0.5)*2 + 505
					case "duration_ms":
						val = math.Sin(vi64*0.1+seed64)*300 +
							math.Sin(vi64*0.03+seed64*2)*200 +
							(rng.Float64()-0.5)*2 + 505
					case "score":
						val = math.Sin(vi64*0.1+seed64)*30 +
							math.Sin(vi64*0.03+seed64*2)*20 +
							(rng.Float64()-0.5)*2 + 51
					case "rate":
						val = math.Sin(vi64*0.1+seed64)*30 +
							math.Sin(vi64*0.03+seed64*2)*20 +
							(rng.Float64()-0.5)*2 + 51
					default:
						val = math.Sin(vi64*0.1+seed64)*30 +
							math.Sin(vi64*0.03+seed64*2)*20 +
							(rng.Float64()-0.5)*2 + 51
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
	log.Info().Msg("=== Tracker Summary ===")
	for _, t := range allTrackers {
		log.Info().Int64("id", t.id).Str("visibility", t.visibility).
			Str("owner", t.ownerName).Msg("tracker")
	}
	log.Info().Int("total", len(allTrackers)).Msg("=== End Summary ===")
	return nil
}
