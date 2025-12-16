// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Department of Linguistic,
//                Faculty of Arts, Charles University
//  This file is part of CNC-MASM.
//
//  CNC-MASM is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  CNC-MASM is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with CNC-MASM.  If not, see <https://www.gnu.org/licenses/>.

package liveattrs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/czcorpus/cnc-gokit/httpclient"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// LiveAttrsJobInfo is a pruned version of Frodo's job info type.
type LiveAttrsJobInfo struct {
	ID       string `json:"id"`
	Finished bool   `json:"finished"`
	Error    string `json:"error,omitempty"`
	OK       bool   `json:"ok"`
}

// --------

type LiveAttrsActions struct {
	conf         LAConf
	frodoURL     *url.URL
	kontextSRURL *url.URL
	httpClient   *http.Client
	ctx          context.Context
}

func (la *LiveAttrsActions) checkJobStatus(url string) (LiveAttrsJobInfo, error) {
	var ans LiveAttrsJobInfo
	resp, err := http.Get(url)
	if err != nil {
		return ans, fmt.Errorf("failed to get info about the job from Frodo: %w", err)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ans, fmt.Errorf("failed to get info about the job from Frodo: %w", err)
	}
	if err := json.Unmarshal(respBody, &ans); err != nil {
		return ans, fmt.Errorf("failed to get info about the job from Frodo: %w", err)
	}
	return ans, nil
}

func (la *LiveAttrsActions) finishJob(jobInfo LiveAttrsJobInfo) error {
	if jobInfo.Error != "" {
		log.Error().
			Str("jobId", jobInfo.ID).
			Str("error", jobInfo.Error).
			Msg("liveattrs job finished with error")
		return fmt.Errorf("liveattrs job failed: %s", jobInfo.Error)
	}

	// Read the soft-reset token from the filesystem
	// and use it to reset KonText's caches etc.
	tokenBytes, err := os.ReadFile(la.conf.KonTextSoftResetTokenPath)
	if err != nil {
		log.Error().
			Err(err).
			Str("tokenPath", la.conf.KonTextSoftResetTokenPath).
			Msg("failed to read KonText soft-reset token")
		return fmt.Errorf("failed to read soft-reset token: %w", err)
	}
	token := string(tokenBytes)

	// Build the URL with the "key" query parameter
	softResetURL := la.kontextSRURL.String()
	reqURL, err := url.Parse(softResetURL)
	if err != nil {
		return fmt.Errorf("failed to parse soft-reset URL: %w", err)
	}
	q := reqURL.Query()
	q.Set("key", token)
	reqURL.RawQuery = q.Encode()

	// Send POST request to KonText soft-reset endpoint
	req, err := http.NewRequest("POST", reqURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create soft-reset request: %w", err)
	}

	resp, err := la.httpClient.Do(req)
	if err != nil {
		log.Error().
			Err(err).
			Str("url", reqURL.String()).
			Msg("failed to send KonText soft-reset request")
		return fmt.Errorf("failed to send soft-reset request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Error().
			Int("statusCode", resp.StatusCode).
			Str("body", string(bodyBytes)).
			Msg("KonText soft-reset returned non-OK status")
		return fmt.Errorf("soft-reset failed with status %d", resp.StatusCode)
	}

	log.Info().
		Str("jobId", jobInfo.ID).
		Msg("successfully triggered KonText soft-reset after liveattrs job completion")
	return nil
}

func (la *LiveAttrsActions) watchJob(jobID string) {
	chkURL, err := url.JoinPath(la.frodoURL.String(), "jobs", jobID)
	if err != nil {
		log.Error().Err(fmt.Errorf("failed to watch liveattrs job: %w", err)).Send()
		return
	}

	// Progressive backoff parameters
	const (
		initialInterval = 5 * time.Second
		maxInterval     = 60 * time.Second
		multiplier      = 1.05
	)

	interval := initialInterval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Debug().
				Str("jobId", jobID).
				Dur("interval", interval).
				Msg("checking liveattrs job status")

			status, err := la.checkJobStatus(chkURL)
			if err != nil {
				log.Error().
					Err(err).
					Str("jobId", jobID).
					Msg("failed to get info about the job from Frodo")
				continue
			}

			if status.Finished {
				log.Info().
					Str("jobId", jobID).
					Bool("ok", status.OK).
					Msg("liveattrs job finished")

				if status.OK {
					if err := la.finishJob(status); err != nil {
						log.Error().
							Err(err).
							Str("jobId", jobID).
							Msg("failed to finish liveattrs job")
					}

				} else {
					log.Error().
						Err(fmt.Errorf(status.Error)).
						Str("jobId", jobID).
						Msg("job finished with error, KonText will not be restarted")
				}
				return
			}
			// Increase interval for next check (progressive backoff)
			newInterval := min(time.Duration(float64(interval)*multiplier), maxInterval)
			if newInterval != interval {
				interval = newInterval
				ticker.Reset(interval)
			}

		case <-la.ctx.Done():
			log.Info().Str("jobId", jobID).Msg("liveattrs job watch cancelled")
		}
	}
}

// Create is a wrapper for Frodo's /liveAttributes/create combined
// with asynchronous waiting loop which - once a respective job
// is finished - calls KonText's "soft-reset".
func (la *LiveAttrsActions) Create(ctx *gin.Context) {
	reqBody, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	buf := bytes.NewBuffer(reqBody)
	req, err := http.NewRequest("POST", la.frodoURL.String(), buf)
	req.URL.Path = ctx.Request.URL.Path
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	resp, err := la.httpClient.Do(req)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	if resp.StatusCode >= 400 {
		// let's try to parse the response as json first:
		var errBody any
		if err := json.Unmarshal(respBody, &errBody); err != nil {
			// this is not necessarily an error - the backend might have e.g. responsed by a non-json response
			errBody = map[string]any{
				"message": string(respBody),
			}
		}
		uniresp.WriteCustomJSONErrorResponse(ctx.Writer, errBody, resp.StatusCode)
		return
	}

	var respData LiveAttrsJobInfo
	err = json.Unmarshal(respBody, &respData)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	if !respData.Finished && respData.Error == "" {
		log.Info().Str("jobId", respData.ID).Msg("going to watch liveattrs job")
		go la.watchJob(respData.ID)
	}
	uniresp.WriteRawJSONResponse(ctx.Writer, respBody)
}

func (la *LiveAttrsActions) Jobs(ctx *gin.Context) {
	targetURL, err := url.Parse(la.frodoURL.String())
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	targetURL.Path = ctx.Request.URL.Path
	targetURL.RawQuery = ctx.Request.URL.RawQuery

	req, err := http.NewRequest(ctx.Request.Method, targetURL.String(), nil)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	for name, values := range ctx.Request.Header {
		if name == "Host" || name == "Connection" {
			continue
		}
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	resp, err := la.httpClient.Do(req)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	for name, values := range resp.Header {
		for _, value := range values {
			ctx.Writer.Header().Add(name, value)
		}
	}

	ctx.Writer.WriteHeader(resp.StatusCode)
	ctx.Writer.Write(respBody)
}

func NewLiveAttrsActions(ctx context.Context, conf LAConf) (*LiveAttrsActions, error) {
	furl, err := url.Parse(conf.FrodoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Frodo URL: %w", err)
	}
	kurl, err := url.Parse(conf.KonTextSoftResetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse KonText soft-reset URL: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = httpclient.TransportMaxIdleConns
	transport.MaxConnsPerHost = httpclient.TransportMaxConnsPerHost
	transport.MaxIdleConnsPerHost = httpclient.TransportMaxIdleConnsPerHost
	transport.IdleConnTimeout = time.Duration(conf.IdleConnTimeoutSecs) * time.Second

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout:   time.Duration(conf.ReqTimeoutSecs) * time.Second,
		Transport: transport,
	}

	return &LiveAttrsActions{
		ctx:          ctx,
		conf:         conf,
		frodoURL:     furl,
		kontextSRURL: kurl,
		httpClient:   client,
	}, nil
}
