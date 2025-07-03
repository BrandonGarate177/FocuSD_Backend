package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2/google"
)

type Session struct {
	ID        string `json:"id"`
	StartTime int64  `json:"startTime"`
	Config    struct {
		Duration      int      `json:"duration"`
		BreakInterval int      `json:"breakInterval"`
		Cycles        int      `json:"cycles"`
		Goal          string   `json:"goal"`
		Tags          []string `json:"tags"`
	} `json:"config"`
	Logs []struct {
		Timestamp  int64   `json:"timestamp"`
		Status     string  `json:"status"`
		Confidence float64 `json:"confidence"`
	} `json:"logs"`
	EndTime int64 `json:"endTime"`
}

type TimeBucket struct {
	Start    int64   `json:"start"`
	Duration int64   `json:"duration"`
	Ratio    float64 `json:"ratio"`
}

type Analysis struct {
	TotalDuration          int64        `json:"totalDuration"`
	AttentiveDuration      int64        `json:"attentiveDuration"`
	AttentionRatio         float64      `json:"attentionRatio"`
	DistractionCount       int          `json:"distractionCount"`
	AvgDistractionDuration float64      `json:"avgDistractionDuration"`
	TimeSeries             []TimeBucket `json:"timeSeries"`
	StartCycleSlump        bool         `json:"startCycleSlump"`
	EndCycleFatigue        bool         `json:"endCycleFatigue"`
	Summary                string       `json:"summary"`
}

func AnalyzeHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var sess Session
		if err := c.BindJSON(&sess); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
			return
		}

		total := sess.EndTime - sess.StartTime
		var attentive, discTotal int64
		var discCount int
		var inDisc bool
		var discStart int64

		for i, log := range sess.Logs {
			nextTs := sess.EndTime
			if i+1 < len(sess.Logs) {
				nextTs = sess.Logs[i+1].Timestamp
			}
			delta := nextTs - log.Timestamp
			if log.Status == "attentive" {
				attentive += delta
				if inDisc {
					discTotal += log.Timestamp - discStart
					discCount++
					inDisc = false
				}
			} else {
				if !inDisc {
					discStart = log.Timestamp
					inDisc = true
				}
			}
		}
		if inDisc {
			discTotal += sess.EndTime - discStart
			discCount++
		}

		avgDisc := 0.0
		if discCount > 0 {
			avgDisc = float64(discTotal) / float64(discCount)
		}
		ratio := 0.0
		if total > 0 {
			ratio = float64(attentive) / float64(total)
		}

		bucketMs := int64(60 * 1000)
		buckets := int(math.Ceil(float64(total) / float64(bucketMs)))
		ts := make([]TimeBucket, buckets)

		for i := 0; i < buckets; i++ {
			start := sess.StartTime + int64(i)*bucketMs
			end := start + bucketMs
			var att, tot int64
			for j, log := range sess.Logs {
				if log.Timestamp < start || log.Timestamp >= end {
					continue
				}
				nextTs := end
				if j+1 < len(sess.Logs) && sess.Logs[j+1].Timestamp < end {
					nextTs = sess.Logs[j+1].Timestamp
				}
				d := nextTs - log.Timestamp
				tot += d
				if log.Status == "attentive" {
					att += d
				}
			}
			r := 0.0
			if tot > 0 {
				r = float64(att) / float64(tot)
			}
			ts[i] = TimeBucket{Start: start, Duration: bucketMs, Ratio: r}
		}

		slump := false
		fatigue := false
		if len(ts) > 0 {
			if ts[0].Ratio < 0.8*ratio {
				slump = true
			}
			if ts[len(ts)-1].Ratio < 0.8*ratio {
				fatigue = true
			}
		}

		analysis := Analysis{
			TotalDuration:          total,
			AttentiveDuration:      attentive,
			AttentionRatio:         ratio,
			DistractionCount:       discCount,
			AvgDistractionDuration: avgDisc,
			TimeSeries:             ts,
			StartCycleSlump:        slump,
			EndCycleFatigue:        fatigue,
		}

		// choose summary source: service account or API key
		log.Printf("Env GAC: %s", os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
		log.Printf("Env GEMINI_API_KEY: %s", os.Getenv("GEMINI_API_KEY"))
		if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" {
			log.Println("Using service account authentication branch")
			if s := generateSummaryWithServiceAccount(analysis); s != "" {
				analysis.Summary = s
			} else {
				analysis.Summary = defaultSummary(ratio, discCount, avgDisc)
			}
		} else if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
			log.Println("Using API key authentication branch")
			if s := generateSummaryWithAPIKey(analysis, apiKey); s != "" {
				analysis.Summary = s
			} else {
				analysis.Summary = defaultSummary(ratio, discCount, avgDisc)
			}
		} else {
			log.Println("No authentication env detected; using default summary")
			analysis.Summary = defaultSummary(ratio, discCount, avgDisc)
		}

		c.JSON(http.StatusOK, analysis)
	}
}

func defaultSummary(ratio float64, discCount int, avgDisc float64) string {
	return fmt.Sprintf("Overall attention %.1f%%, %d distractions (avg %.1f s)", ratio*100, discCount, avgDisc/1000.0)
}

// generateSummaryWithAPIKey sends request using an API key
func generateSummaryWithAPIKey(a Analysis, apiKey string) string {
	prompt := fmt.Sprintf(`Here are your study session metrics:
- Total duration: %.1f minutes
- Attentive duration: %.1f minutes
- Attention ratio: %.1f%%
- Distraction count: %d
- Average distraction duration: %.1f seconds
- Start cycle slump: %t
- End cycle fatigue: %t

Please write a friendly, supportive, and encouraging summary of this session in 2-3 sentences. Use positive language, highlight any achievements or improvements, and offer gentle motivation for next time. Refer to the user as 'you' and keep the tone uplifting.`,
		float64(a.TotalDuration)/60000.0,
		float64(a.AttentiveDuration)/60000.0,
		a.AttentionRatio*100,
		a.DistractionCount,
		a.AvgDistractionDuration/1000.0,
		a.StartCycleSlump,
		a.EndCycleFatigue,
	)

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": []map[string]string{
				{"text": prompt},
			}},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	req, err := http.NewRequest("POST", "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key="+apiKey, bytes.NewBuffer(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("APIKey request error: %v", err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("APIKey non-OK HTTP %d: %s", resp.StatusCode, string(body))
		return ""
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return ""
	}
	return result.Candidates[0].Content.Parts[0].Text
}

// generateSummaryWithServiceAccount uses service account credentials for auth
func generateSummaryWithServiceAccount(a Analysis) string {
	ctx := context.Background()
	credPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	log.Printf("GAC path from env: %s", credPath)
	var absPath string
	if filepath.IsAbs(credPath) {
		absPath = credPath
	} else {
		wd, _ := os.Getwd()
		absPath = filepath.Join(wd, credPath)
	}
	log.Printf("Resolved GAC absolute path: %s", absPath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		log.Printf("failed reading creds file: %v", err)
		return ""
	}
	conf, err := google.JWTConfigFromJSON(data, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		log.Printf("invalid creds JSON: %v", err)
		return ""
	}
	client := conf.Client(ctx)
	// build prompt same as APIKey version
	prompt := fmt.Sprintf(`Here are your study session metrics:
- Total duration: %.1f minutes
- Attentive duration: %.1f minutes
- Attention ratio: %.1f%%
- Distraction count: %d
- Average distraction duration: %.1f seconds
- Start cycle slump: %t
- End cycle fatigue: %t

Please write a friendly, supportive, and encouraging summary of this session in 2-3 sentences. Use positive language, highlight any achievements or improvements, and offer gentle motivation for next time. Refer to the user as 'you' and keep the tone uplifting.`,
		float64(a.TotalDuration)/60000.0,
		float64(a.AttentiveDuration)/60000.0,
		a.AttentionRatio*100,
		a.DistractionCount,
		a.AvgDistractionDuration/1000.0,
		a.StartCycleSlump,
		a.EndCycleFatigue,
	)
	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": []map[string]string{
				{"text": prompt},
			}},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	req, err := http.NewRequest("POST", "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent", bytes.NewBuffer(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ServiceAccount request error: %v", err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("ServiceAccount non-OK HTTP %d: %s", resp.StatusCode, string(body))
		return ""
	}
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return ""
	}
	return result.Candidates[0].Content.Parts[0].Text
}
