package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
)

type HttpClient interface {
	GetCommits() ([]GitHttpCommitResponse, error)
}

type httpClient struct{}

func NewHttpClient() HttpClient {
	return &httpClient{}
}

func (c *httpClient) GetCommits() ([]GitHttpCommitResponse, error) {
	request, err := http.NewRequest(http.MethodGet, DefaultGithubUrl, nil)
	if err != nil {
		return nil, err
	}

	gitToken, ok := os.LookupEnv("GITHUB_TOKEN")
	if !ok {
		return nil, errors.New("GITHUB_TOKEN environment variable not set")
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+gitToken)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	statusCode := response.StatusCode

	if statusCode != 200 {
		return nil, fmt.Errorf("unable to fetch remote commit: %v", statusCode)
	}

	var responseBody []GitHttpCommitResponse

	err = json.NewDecoder(response.Body).Decode(&responseBody)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return responseBody, nil
}
