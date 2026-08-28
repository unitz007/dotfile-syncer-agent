package main

import "testing"

func TestParseGitUrl(t *testing.T) {
	tests := []struct {
		name      string
		gitURL    string
		wantOwner string
		wantRepo  string
	}{
		{
			name:      "https url",
			gitURL:    "https://github.com/unitz007/dotfiles.git",
			wantOwner: "unitz007",
			wantRepo:  "dotfiles",
		},
		{
			name:      "ssh scp-style url",
			gitURL:    "git@github.com:unitz007/dotfiles.git",
			wantOwner: "unitz007",
			wantRepo:  "dotfiles",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := ParseGitUrl(tt.gitURL)
			if err != nil {
				t.Fatalf("ParseGitUrl() error = %v", err)
			}
			if owner != tt.wantOwner || repo != tt.wantRepo {
				t.Fatalf("ParseGitUrl() = %s, %s; want %s, %s", owner, repo, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}
