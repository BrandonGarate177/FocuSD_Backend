package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"math"
	"net/http"
	"os"
)

type Session struct {
	ID        string `json:"id"`
	StartTime int64  `json:"startTime"`
	Config    struct {
		Duration      int      `json:"duration"` // minutes
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

// TimeBucket is one point in the sparkline/heatmap
type TimeBucket struct {
	Start    int64   `json:"start"`    // bucket start timestamp
	Duration int64   `json:"duration"` // bucket width (ms)
	Ratio    float64 `json:"ratio"`    // attentive ratio
}

// Analysis holds computed metrics for a session
type Analysis struct {
	TotalDuration          int64        `json:"totalDuration"`     // ms
	AttentiveDuration      int64        `json:"attentiveDuration"` // ms
	AttentionRatio         float64      `json:"attentionRatio"`    // 0-1
	DistractionCount       int          `json:"distractionCount"`
	AvgDistractionDuration float64      `json:"avgDistractionDuration"` // ms
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
		// compute durations
		total := sess.EndTime - sess.StartTime
		var attentive int64
		var discCount int
		var discTotal int64
		// compute durations by iterating logs
		var inDisc bool
		var discStart int64
		for i, log := range sess.Logs {
			// determine next timestamp
			nextTs := sess.EndTime
			if i+1 < len(sess.Logs) {
				nextTs = sess.Logs[i+1].Timestamp
			}
			delta := nextTs - log.Timestamp
			if log.Status == "attentive" {
				attentive += delta
				if inDisc {
					// end of distraction
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
		// finalize last distraction if still open
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
		// bucket per minute
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
		// detect slump/fatigue (first and last bucket ratio <80% of overall)
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
		// assemble metrics
		analysis := Analysis{
			TotalDuration:          total,
			AttentiveDuration:      attentive,
			AttentionRatio:         ratio,
			DistractionCount:       discCount,
			AvgDistractionDuration: avgDisc,
			TimeSeries:             ts,
			StartCycleSlump:        slump,
			EndCycleFatigue:        fatigue,
			Summary:                "",
		}
		// generate LLM-driven summary if key present
		if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
			analysis.Summary = generateSummary(analysis)
		} else {
			analysis.Summary = fmt.Sprintf(
				"Overall attention %.1f%%, %d distractions (avg %.1f s)",
				ratio*100, discCount, avgDisc/1000.0,
			)
		}
		c.JSON(http.StatusOK, analysis)
	}
}

func generateSummary(a Analysis) string {
	apiKey := os.Getenv("GEMINI_API_KEY")

	prompt := fmt.Sprintf(
		"Given the following study session metrics:\n"+
			"- Total duration: %.1f minutes\n"+
			"- Attentive duration: %.1f minutes\n"+
			"- Attention ratio: %.1f%%\n"+
			"- Distraction count: %d\n"+
			"- Average distraction duration: %.1f seconds\n"+
			"- Start cycle slump: %t\n"+
			"- End cycle fatigue: %t\n"+
			"Summarize the type of session the user had in 2-3 sentences. Clearly state if the user was focused, distracted, or fatigued, and mention any notable patterns. Refer to the user with 'you' and use a friendly, encouraging tone.\n\n",
		float64(a.TotalDuration)/60000.0,
		float64(a.AttentiveDuration)/60000.0,
		a.AttentionRatio*100,
		a.DistractionCount,
		a.AvgDistractionDuration/1000.0,
		a.StartCycleSlump,
		a.EndCycleFatigue,
	)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"safetySettings": []map[string]interface{}{
			{"category": "HARM_CATEGORY_DANGEROUS_CONTENT", "threshold": 4},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.7,
			"maxOutputTokens": 150,
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=" + apiKey
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "error creating request"
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "error sending request"
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	_ = json.Unmarshal(body, &res)
	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		return res.Candidates[0].Content.Parts[0].Text
	}
	return "no response"
}
