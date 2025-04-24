package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDoRun(t *testing.T) {
	tests := []struct {
		name string
		args string
		want *RunCommandResponse
		err  error
	}{
		// good
		{
			name: "simple",
			args: "echo hello",
			want: &RunCommandResponse{
				Stdout: "hello\n",
			},
		},
		{
			name: "pipe",
			args: "echo hello | wc -l",
			want: &RunCommandResponse{
				Stdout: "       1\n",
			},
		},
		{
			name: "pipe twice",
			args: "echo hello | wc -l | xargs",
			want: &RunCommandResponse{
				Stdout: "1\n",
			},
		},
		// bad
		{
			name: "command not found",
			args: "bleh",
			want: nil,
			err:  fmt.Errorf("error starting command \"bleh\": exec: \"bleh\": executable file not found in $PATH"),
		},
		{
			name: "bad quoting",
			args: "echo 'hello",
			want: nil,
			err:  fmt.Errorf("error parsing command \"echo 'hello\": EOF found when expecting closing quote"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseAndRun(test.args)
			require.Equal(t, test.err, err)
			if test.want != nil {
				require.NotNil(t, got)
				require.Equal(t, *test.want, *got)
			}
		})
	}
}
