package main

import (
	"testing"
)

func TestExecutionPlan_Validate(t *testing.T) {
	tests := []struct {
		name    string
		plan    *ExecutionPlan
		wantErr bool
	}{
		{
			name:    "Nil plan is valid",
			plan:    nil,
			wantErr: false,
		},
		{
			name: "Valid plan with only install",
			plan: &ExecutionPlan{
				Install: []string{"apt-get install git"},
			},
			wantErr: false,
		},
		{
			name: "Valid plan with files and repo",
			plan: &ExecutionPlan{
				DotfilesRepo: "https://github.com/user/dotfiles",
				Files: []FileMapping{
					{Source: ".bashrc", Target: "~/.bashrc"},
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid plan with files but no repo",
			plan: &ExecutionPlan{
				Files: []FileMapping{
					{Source: ".bashrc", Target: "~/.bashrc"},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid file mapping missing source",
			plan: &ExecutionPlan{
				DotfilesRepo: "https://github.com/user/dotfiles",
				Files: []FileMapping{
					{Source: "", Target: "~/.bashrc"},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid file mapping missing target",
			plan: &ExecutionPlan{
				DotfilesRepo: "https://github.com/user/dotfiles",
				Files: []FileMapping{
					{Source: ".bashrc", Target: ""},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.plan.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("ExecutionPlan.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
