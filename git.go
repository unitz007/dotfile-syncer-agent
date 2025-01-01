package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

type Git struct {
	config *Configurations
}

func (g Git) RemoteCommit() (*Commit, error) {

	gitUrl, err := url.Parse(fmt.Sprintf("https://api.github.com/repos/%s/%s/commits", g.config.RepositoryOwner, g.config.GitRepository))
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(http.MethodGet, gitUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	gitToken := g.config.GithubToken
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

	headCommit := responseBody[0]

	commit := &Commit{
		Id:   headCommit.Sha,
		Time: headCommit.Commit.Author.Date,
	}

	return commit, nil
}

func (g Git) LocalCommit() (*Commit, error) {
	err := os.Chdir(g.config.DotfilePath)
	if err != nil {
		return nil, err
	}

	path, err := exec.LookPath("git")
	if err != nil {
		return nil, err
	}

	headCommit, err := exec.Command(path, "log", "HEAD", "-1").CombinedOutput()
	if err != nil {
		return nil, err
	}

	commitDetails := strings.Split(string(headCommit), "\n")

	commitId := strings.Split(commitDetails[0], " ")[1]
	commitTime := func() string {
		n := strings.Replace(commitDetails[2], "Date:  ", "", 1)
		return strings.TrimSpace(n)
	}()

	commit := &Commit{
		Id:   commitId,
		Time: commitTime,
	}
	return commit, nil
}

func (g Git) IsSync() bool {
	localCommit, err := g.LocalCommit()
	remoteCommit, err := g.RemoteCommit()
	if err != nil {
		return false
	}

	return localCommit.Id == remoteCommit.Id
}
