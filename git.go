package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
)

// Git provides operations for interacting with Git repositories and the GitHub API
type Git struct {
	config *Configurations // Agent configuration containing repository details
}

// RemoteCommit fetches the latest commit from the remote GitHub repository using the GitHub API.
// Returns the commit SHA and timestamp of the HEAD commit on the default branch.
func (g Git) RemoteCommit() (*Commit, error) {
	gitUrl, err := url.Parse(fmt.Sprintf("%s/repos/%s/%s/commits", g.config.GitApiBaseUrl, g.config.RepositoryOwner, g.config.GitRepository))
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

// LocalCommit retrieves the latest commit from the local Git repository.
// It executes 'git log HEAD -1' to get the most recent commit information.
func (g Git) LocalCommit() (*Commit, error) {
	err := os.Chdir(g.config.DotfilePath + string(os.PathSeparator) + g.config.GitRepository)
	if err != nil {
		return nil, err
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, err
	}

	headCommit, err := exec.Command(gitPath, "log", "HEAD", "-1").CombinedOutput()
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

// IsSync compares local and remote commits to determine if they are synchronized.
// Returns true if both commits have the same SHA, false otherwise.
func (g Git) IsSync(localCommit, remoteCommit *Commit) bool {

	if localCommit == nil || remoteCommit == nil {
		return false
	}

	return localCommit.Id == remoteCommit.Id
}

// CloneOrPullRepository clones the repository if it doesn't exist locally,
// or pulls the latest changes if it already exists.
// This ensures the local repository is up-to-date with the remote.
func (g Git) CloneOrPullRepository() error {

	git, err := exec.LookPath("git")
	if err != nil {
		return err
	}

	return func() error {
		repoPath := path.Join(g.config.DotfilePath, g.config.GitRepository)
		_, err = os.Stat(repoPath) // checks if repo already exists
		if err != nil {
			// Repository doesn't exist, clone it
			err = os.Chdir(g.config.DotfilePath)
			if err != nil {
				return err
			}

			err = exec.Command(git, "clone", g.config.GitUrl).Run()
			if err != nil {
				return err
			}

			return os.Chdir(g.config.GitRepository)
		} else {
			// Repository exists, pull latest changes
			err = os.Chdir(repoPath)
			if err != nil {
				return err
			}

			return exec.Command(git, "pull", "origin", "main").Run()

		}
	}()
}
