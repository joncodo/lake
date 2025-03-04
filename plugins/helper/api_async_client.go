/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/apache/incubator-devlake/plugins/core"
	"github.com/apache/incubator-devlake/plugins/helper/common"
	"github.com/apache/incubator-devlake/utils"
)

var HttpMinStatusRetryCode = http.StatusBadRequest

// ApiAsyncClient is built on top of ApiClient, to provide a asynchronous semantic
// You may submit multiple requests at once by calling `GetAsync`, and those requests
// will be performed in parallel with rate-limit support
type ApiAsyncClient struct {
	*ApiClient
	maxRetry     int
	scheduler    *WorkerScheduler
	hasError     bool
	numOfWorkers int
}

// CreateAsyncApiClient creates a new ApiAsyncClient
func CreateAsyncApiClient(
	taskCtx core.TaskContext,
	apiClient *ApiClient,
	rateLimiter *ApiRateLimitCalculator,
) (*ApiAsyncClient, error) {
	// load retry/timeout from configuration
	retry, err := utils.StrToIntOr(taskCtx.GetConfig("API_RETRY"), 3)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API_RETRY: %w", err)
	}
	timeout, err := utils.StrToDurationOr(taskCtx.GetConfig("API_TIMEOUT"), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API_TIMEOUT: %w", err)
	}
	apiClient.SetTimeout(timeout)
	apiClient.SetLogger(taskCtx.GetLogger())
	globalRateLimitPerHour, err := utils.StrToIntOr(taskCtx.GetConfig("API_REQUESTS_PER_HOUR"), 18000)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API_REQUESTS_PER_HOUR: %w", err)
	}
	if rateLimiter == nil {
		rateLimiter = &ApiRateLimitCalculator{}
	}
	rateLimiter.GlobalRateLimitPerHour = globalRateLimitPerHour
	rateLimiter.MaxRetry = retry

	// ok, calculate api rate limit based on response (normally from headers)
	requests, duration, err := rateLimiter.Calculate(apiClient)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate rateLimit for api: %w", err)
	}

	// it is hard to tell how many workers would be sufficient, it depends on how slow the server responds.
	// we need more workers when server is responding slowly, because requests are sent in a fixed pace.
	// and because workers are relatively cheap, lets assume response takes 5 seconds
	const RESPONSE_TIME = 5 * time.Second
	// in order for scheduler to hold requests of 3 seconds, we need:
	d := duration / RESPONSE_TIME
	numOfWorkers := requests / int(d)

	logger := taskCtx.GetLogger()
	logger.Info(
		"scheduler for api %s worker: %d, request: %d, duration: %v",
		apiClient.GetEndpoint(),
		numOfWorkers,
		requests,
		duration,
	)
	scheduler, err := NewWorkerScheduler(
		numOfWorkers,
		requests,
		duration,
		taskCtx.GetContext(),
		retry,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	// finally, wrap around api client with async sematic
	return &ApiAsyncClient{
		apiClient,
		retry,
		scheduler,
		false,
		numOfWorkers,
	}, nil
}

// GetMaxRetry returns the maximum retry attempts for a request
func (apiClient *ApiAsyncClient) GetMaxRetry() int {
	return apiClient.maxRetry
}

// SetMaxRetry sets the maximum retry attempts for a request
func (apiClient *ApiAsyncClient) SetMaxRetry(
	maxRetry int,
) {
	apiClient.maxRetry = maxRetry
}

// DoAsync would carry out a asynchronous request
func (apiClient *ApiAsyncClient) DoAsync(
	method string,
	path string,
	query url.Values,
	body interface{},
	header http.Header,
	handler common.ApiAsyncCallback,
	retry int,
) {
	var request func() error
	request = func() error {
		var err error
		var res *http.Response
		var body []byte
		res, err = apiClient.Do(method, path, query, body, header)
		// make sure response body is read successfully, or we might have to retry
		if err == nil {
			// make sure response.Body stream will be closed to avoid running out of file handle
			defer func(body io.ReadCloser) { body.Close() }(res.Body)
			// replace NetworkStream with MemoryBuffer
			body, err = ioutil.ReadAll(res.Body)
			if err == nil {
				res.Body = io.NopCloser(bytes.NewBuffer(body))
			}
		}

		// check
		needRetry := false
		if err != nil {
			needRetry = true
		} else if res.StatusCode >= HttpMinStatusRetryCode {
			needRetry = true
			err = fmt.Errorf("http code error[%d]:[%s]", res.StatusCode, body)
		}

		//  if it need retry, check and try to retry
		if needRetry {
			// check weather we still have retry times and not error from handler and canceled error
			if retry < apiClient.maxRetry && err != context.Canceled {
				apiClient.logger.Warn("retry #%d for %s", retry, err.Error())
				retry++
				apiClient.scheduler.NextTick(request)
				return nil
			}
		}

		if err != nil {
			apiClient.hasError = true
			return err
		}

		// it is important to let handler have a chance to handle error, or it can hang indefinitely
		// when error occurs
		return handler(res)
	}
	apiClient.scheduler.SubmitBlocking(request)
}

// Enqueue an api get request, the request may be sent sometime in future in parallel with other api requests
func (apiClient *ApiAsyncClient) GetAsync(
	path string,
	query url.Values,
	header http.Header,
	handler common.ApiAsyncCallback,
) {
	apiClient.DoAsync(http.MethodGet, path, query, nil, header, handler, 0)
}

// WaitAsync blocks until all async requests were done
func (apiClient *ApiAsyncClient) WaitAsync() error {
	return apiClient.scheduler.Wait()
}

func (apiClient *ApiAsyncClient) HasError() bool {
	return apiClient.hasError
}

func (apiClient *ApiAsyncClient) NextTick(task func() error) {
	apiClient.scheduler.NextTick(task)
}

func (apiClient *ApiAsyncClient) GetNumOfWorkers() int {
	return apiClient.numOfWorkers
}

type RateLimitedApiClient interface {
	GetAsync(path string, query url.Values, header http.Header, handler common.ApiAsyncCallback)
	WaitAsync() error
	HasError() bool
	NextTick(task func() error)
	GetNumOfWorkers() int
}

var _ RateLimitedApiClient = (*ApiAsyncClient)(nil)
